//go:build linux

package terminal

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/omartelo/lich/internal/events"
)

// TestSessionAccountReadsWhatTheWrapperExecsInto is the mechanism the whole
// per-session quota reading rests on: a configured binary that exports a login
// of its own and `exec`s the provider replaces its own image, and what lich
// reads back is the environment the provider actually runs with — not the one
// lich spawned the wrapper with.
func TestSessionAccountReadsWhatTheWrapperExecsInto(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "claude-work.sh")
	script := "#!/bin/sh\nexport CLAUDE_CONFIG_DIR=" + dir + "\nexec sleep 30\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatalf("write wrapper: %v", err)
	}
	svc := New(stubBins{bin: bin}, nil, events.New())
	t.Cleanup(func() { _ = svc.Close("s1") })
	if err := svc.Start("s1", "p1", t.TempDir(), "claude", "", "", false, false, 80, 24); err != nil {
		t.Fatalf("Start = %v, want nil", err)
	}

	// The export lands when the wrapper execs, which is after the spawn returns.
	var env map[string]string
	var read bool
	for range 200 {
		env, _, read = svc.SessionAccount("s1")
		if read && env["CLAUDE_CONFIG_DIR"] == dir {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if !read {
		t.Fatal("read = false, want the live process's environment")
	}
	if got := env["CLAUDE_CONFIG_DIR"]; got != dir {
		t.Errorf("CLAUDE_CONFIG_DIR = %q, want %q — the wrapper's own login", got, dir)
	}
}
