//go:build linux

package terminal

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/omartelo/lich/internal/events"
	"github.com/omartelo/lich/internal/sandbox"
)

// A confined session is still a PTY session: bubblewrap sits between lich and
// the shell, and everything the terminal does — reading output, writing at the
// prompt, reaping the exit — has to keep working through it. The rest of the
// suite proves the sandbox confines; this proves a card still opens.
func TestAConfinedSessionRunsInItsPTY(t *testing.T) {
	if !sandbox.Available() {
		t.Skip("no sandbox backend on this machine")
	}
	t.Setenv("SHELL", "sh")
	home := t.TempDir()
	t.Setenv("HOME", home)
	cwd := filepath.Join(home, "checkout")
	// A sibling of the checkout, inside the same home: what the private home has
	// to leave behind while still carrying the directory the session works in.
	for _, dir := range []string{"checkout", "other-work"} {
		if err := os.MkdirAll(filepath.Join(home, dir), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	svc := New(stubBins{sandboxOn: true}, nil, events.New())
	t.Cleanup(func() { _ = svc.Close("s1") })

	if err := svc.Start("s1", "p1", cwd, KindShell, "", "", false, false, 80, 24); err != nil {
		t.Fatalf("Start = %v, want nil", err)
	}
	// Both markers are assembled from two shell variables, so the PTY's echo of
	// the command carries `$A$B` and only the shell's own output carries "HOME:"
	// and "END". Matching the echo would pass this test on a sandbox that never
	// ran, and the trailing marker is what says the answer is complete — the
	// listing has no newline of its own.
	if err := svc.Write("s1", `A=HO; B=ME; ls -A "$HOME" | tr '\n' ' ' | sed "s/^/$A$B: /;s/$/$B$A/"`+"\n"); err != nil {
		t.Fatalf("Write = %v, want nil", err)
	}

	var answer string
	waitFor(t, func() bool {
		encoded, err := svc.Replay("s1")
		if err != nil {
			return false
		}
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return false
		}
		at := strings.LastIndex(string(decoded), "HOME: ")
		if at < 0 {
			return false
		}
		answer = string(decoded)[at:]
		return strings.Contains(answer, "MEHO")
	}, "a confined session to answer at its prompt")

	if !strings.Contains(answer, "checkout") {
		t.Errorf("the session's own checkout is missing from its home: %q", answer)
	}
	if strings.Contains(answer, "other-work") {
		t.Errorf("the host home reached into the sandbox: %q", answer)
	}
}

// TestAConfinedSpawnRecordsWhatItsSandboxSkipped closes the journey the links
// take: resolved by wrapSandbox, carried on the session, and written to the row
// by Start. Only the spawn ever resolves them and the PTY outlives the page, so
// a reload has no second spawn to hear them from — a card whose session cannot
// find its own ~/.gitconfig would then have nothing to say about why.
func TestAConfinedSpawnRecordsWhatItsSandboxSkipped(t *testing.T) {
	if !sandbox.Available() {
		t.Skip("no sandbox backend on this machine")
	}
	t.Setenv("SHELL", "sh")
	home := t.TempDir()
	t.Setenv("HOME", home)
	cwd := filepath.Join(home, "checkout")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatalf("mkdir checkout: %v", err)
	}
	// The shape a dotfile manager leaves behind, and the one the sandbox has to
	// drop: a bind of a symlink is what fails the spawn.
	target := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(target, []byte("[user]\n"), 0o600); err != nil {
		t.Fatalf("write link target: %v", err)
	}
	if err := os.Symlink(target, filepath.Join(home, ".gitconfig")); err != nil {
		t.Fatalf("symlink .gitconfig: %v", err)
	}

	recorded := map[string][]string{}
	svc := New(stubBins{sandboxOn: true, sandboxLinks: recorded}, nil, events.New())
	t.Cleanup(func() { _ = svc.Close("s1") })

	if err := svc.Start("s1", "p1", cwd, KindShell, "", "", false, false, 80, 24); err != nil {
		t.Fatalf("Start = %v, want nil", err)
	}

	if !slices.Contains(recorded["s1"], ".gitconfig") {
		t.Errorf("the row recorded %v, want it to name the skipped .gitconfig", recorded["s1"])
	}
}
