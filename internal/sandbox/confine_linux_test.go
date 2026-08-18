//go:build linux

package sandbox

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/omartelo/lich/internal/providers"
)

// confinedHome is a home directory with the shapes the sandbox has to tell
// apart: the provider's state, a build cache, a secret, and two checkouts of
// which only one is the session's.
func confinedHome(t *testing.T) (home, cwd string) {
	t.Helper()
	home = t.TempDir()
	for _, dir := range []string{".claude", ".cache", ".ssh", "work/repo", "work/other-repo"} {
		if err := os.MkdirAll(filepath.Join(home, dir), 0o700); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	if err := os.WriteFile(filepath.Join(home, ".ssh", "id_ed25519"), []byte("secret"), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	return home, filepath.Join(home, "work", "repo")
}

// runConfined runs one shell line inside the sandbox and returns its combined
// output. The shell is /bin/sh, which the read-only base filesystem provides.
func runConfined(t *testing.T, home, cwd, script string) string {
	t.Helper()
	spec := Describe(providers.Claude, home, cwd, "", nil)
	bin, args := Wrap(spec, "/bin/sh", []string{"-c", script})
	out, err := exec.Command(bin, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("confined run failed: %v\n%s", err, out)
	}
	return string(out)
}

// This is the whole promise of the feature, checked against a real bubblewrap:
// the provider's state is there, the secret and the sibling checkout are not.
func TestConfinedSessionSeesOnlyItsOwnWork(t *testing.T) {
	if !Available() {
		t.Skip("bubblewrap is not installed")
	}
	home, cwd := confinedHome(t)
	out := runConfined(t, home, cwd, `ls -A "$HOME"; echo ---; ls -A "$HOME/work"`)

	for _, want := range []string{".claude", ".cache", "work"} {
		if !strings.Contains(out, want) {
			t.Errorf("%s is missing from the confined home:\n%s", want, out)
		}
	}
	if strings.Contains(out, ".ssh") {
		t.Errorf("the ssh key directory is reachable inside the sandbox:\n%s", out)
	}
	if strings.Contains(out, "other-repo") {
		t.Errorf("a sibling checkout is reachable inside the sandbox:\n%s", out)
	}
	if !strings.Contains(out, "repo") {
		t.Errorf("the session's own checkout is missing:\n%s", out)
	}
}

func TestConfinedSessionWritesOnlyWhereItWorks(t *testing.T) {
	if !Available() {
		t.Skip("bubblewrap is not installed")
	}
	home, cwd := confinedHome(t)
	// Every touch is guarded, so the script itself always exits 0 and the
	// verdicts arrive as output rather than as a failed run.
	out := runConfined(t, home, cwd, strings.Join([]string{
		`echo "checkout: $(touch ./probe 2>/dev/null && echo ok || echo denied)"`,
		`echo "state: $(touch "$HOME/.claude/probe" 2>/dev/null && echo ok || echo denied)"`,
		`echo "cache: $(touch "$HOME/.cache/probe" 2>/dev/null && echo ok || echo denied)"`,
		`echo "system: $(touch /etc/probe 2>/dev/null && echo ok || echo denied)"`,
	}, "; "))

	for _, want := range []string{"checkout: ok", "state: ok", "cache: ok", "system: denied"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in:\n%s", want, out)
		}
	}
	// The writes that succeeded are the ones the host must actually have.
	for _, path := range []string{
		filepath.Join(cwd, "probe"),
		filepath.Join(home, ".claude", "probe"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("a write the sandbox allowed never reached the host: %s", path)
		}
	}
}

// The private home is writable and thrown away with the session: a dotfile an
// agent writes outside its state is gone next spawn, and never touches the host.
func TestConfinedHomeIsEphemeral(t *testing.T) {
	if !Available() {
		t.Skip("bubblewrap is not installed")
	}
	home, cwd := confinedHome(t)
	runConfined(t, home, cwd, `touch "$HOME/.bashrc"`)
	if _, err := os.Stat(filepath.Join(home, ".bashrc")); err == nil {
		t.Error("a write to the private home reached the host home")
	}
	out := runConfined(t, home, cwd, `test -e "$HOME/.bashrc" && echo kept || echo gone`)
	if !strings.Contains(out, "gone") {
		t.Errorf("the private home survived the session:\n%s", out)
	}
}

// installedLikeAnAgent lays out the shape every agent installer uses: a symlink
// on PATH pointing into a versioned store elsewhere in the home. It is the shape
// that broke the first confined spawn — bubblewrap resolves a mount destination
// through symlinks, so binding the link itself fails with "Can't create file".
func installedLikeAnAgent(t *testing.T, home string) string {
	t.Helper()
	store := filepath.Join(home, ".local", "share", "agent", "versions")
	bin := filepath.Join(home, ".local", "bin")
	for _, dir := range []string{store, bin} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	real := filepath.Join(store, "9.9.9")
	if err := os.WriteFile(real, []byte("#!/bin/sh\necho agent-ran\n"), 0o755); err != nil {
		t.Fatalf("write agent: %v", err)
	}
	link := filepath.Join(bin, "agent")
	if err := os.Symlink(real, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	return link
}

func TestAnAgentInstalledAsASymlinkRuns(t *testing.T) {
	if !Available() {
		t.Skip("bubblewrap is not installed")
	}
	home, cwd := confinedHome(t)
	link := installedLikeAnAgent(t, home)

	// PATH is what BinaryDirs resolves a bare name through, so the fixture has
	// to be on it — the same way the user's own installer puts it there.
	t.Setenv("PATH", filepath.Join(home, ".local", "bin")+":"+os.Getenv("PATH"))

	spec := Describe(providers.Claude, home, cwd, "", BinaryDirs("agent"))
	bin, args := Wrap(spec, link, nil)
	out, err := exec.Command(bin, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("a confined spawn of a symlinked agent failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "agent-ran") {
		t.Errorf("the agent did not run: %q", out)
	}
}
