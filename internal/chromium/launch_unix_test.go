//go:build !windows

package chromium

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// standIn is a window that exits at once: enough to walk launch end to end —
// the pin resolving, the keyed profile directory made, the stdin pipe held,
// the exit coming back — without a CEF build. Unix only for the binary's name.
func standIn(t *testing.T) (exe, profile string) {
	t.Helper()
	exe, err := exec.LookPath("true")
	if err != nil {
		t.Skipf("no `true` on PATH: %v", err)
	}
	t.Setenv(OverrideEnv, exe)
	return exe, filepath.Join(t.TempDir(), "chromium-profile")
}

func TestFocusLaunchesThePinnedWindowOnItsProfile(t *testing.T) {
	exe, profile := standIn(t)
	if err := Focus("http://127.0.0.1:1/", profile, "lichtest"); err != nil {
		t.Fatalf("Focus: %v", err)
	}
	keyed := filepath.Join(profile, (Result{Path: exe}).profileKey())
	if info, err := os.Stat(keyed); err != nil || !info.IsDir() {
		t.Fatalf("profile directory %q not created: %v", keyed, err)
	}
}

// Run hands the window process to onStart before waiting on it, which is what
// the restart flow closes; a clean exit is the window closed by the user.
func TestRunHandsTheWindowProcessToOnStart(t *testing.T) {
	_, profile := standIn(t)
	var started *os.Process
	err := Run("http://127.0.0.1:1/", profile, "lichtest", []string{"--ozone-platform=x11"}, func(p *os.Process) { started = p })
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if started == nil {
		t.Fatal("onStart never received the window process")
	}
}
