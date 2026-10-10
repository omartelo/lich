//go:build !windows

package chromium

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
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

// A window that dies at launch says why: its stderr comes back on the exit,
// which is what main.go logs and shows.
func TestFocusReportsWhatTheWindowWroteBeforeItDied(t *testing.T) {
	fatal := "[1:1:0911/1:FATAL:setuid_sandbox_host.cc:166] The SUID sandbox helper binary was found"
	exe := filepath.Join(t.TempDir(), "lich-shell")
	script := "#!/bin/sh\necho 'Fontconfig warning' >&2\necho '" + fatal + "' >&2\nexit 5\n"
	if err := os.WriteFile(exe, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(OverrideEnv, exe)
	err := Focus("http://127.0.0.1:1/", filepath.Join(t.TempDir(), "chromium-profile"), "lichtest")
	var exit *ExitError
	var code *exec.ExitError
	if !errors.As(err, &exit) || !errors.As(err, &code) || code.ExitCode() != 5 {
		t.Fatalf("Focus = %v, want an ExitError on exit status 5", err)
	}
	if want := []string{"Fontconfig warning", fatal}; !slices.Equal(exit.Stderr, want) {
		t.Fatalf("Stderr = %q, want %q", exit.Stderr, want)
	}
}

// Run hands onStart the window's close before waiting on it, which is what
// quitting and restarting close it with: a write on the window's stdin, which
// the stand-in here exits on, as lich-shell does.
func TestRunClosesTheWindowThroughItsStdin(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "lich-shell")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\nhead -c 1 >/dev/null\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(OverrideEnv, exe)
	err := Run("http://127.0.0.1:1/", filepath.Join(t.TempDir(), "chromium-profile"), "lichtest", nil,
		func(close func() error) {
			if err := close(); err != nil {
				t.Errorf("close: %v", err)
			}
		})
	if err != nil {
		t.Fatalf("Run: %v, want the window closed cleanly", err)
	}
}
