package terminal

import (
	"io"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty"
)

// stubBins is a BinResolver returning a fixed path, for tests that never spawn.
type stubBins struct{ bin string }

func (s stubBins) ClaudeBin(string) string { return s.bin }

// TestOperationsOnUnknownSessionAreNoops proves Write/Resize/Close on a session
// that was never started return nil instead of panicking on a missing PTY.
func TestOperationsOnUnknownSessionAreNoops(t *testing.T) {
	svc := New(stubBins{})
	if err := svc.Write("ghost", "hi"); err != nil {
		t.Errorf("Write unknown = %v, want nil", err)
	}
	if err := svc.Resize("ghost", 80, 24); err != nil {
		t.Errorf("Resize unknown = %v, want nil", err)
	}
	if err := svc.Close("ghost"); err != nil {
		t.Errorf("Close unknown = %v, want nil", err)
	}
}

// TestResolveBin proves an empty custom path falls back to the default binary
// while a configured path is passed through unchanged.
func TestResolveBin(t *testing.T) {
	if got := resolveBin(""); got != defaultBin {
		t.Errorf("resolveBin(%q) = %q, want %q", "", got, defaultBin)
	}
	if got := resolveBin("/opt/claude.sh"); got != "/opt/claude.sh" {
		t.Errorf("resolveBin custom = %q, want %q", got, "/opt/claude.sh")
	}
}

// TestPTYEcho proves the core assumption of the service: a process spawns under
// a PTY and its output is readable. If creack/pty breaks, this fails.
func TestPTYEcho(t *testing.T) {
	const marker = "skipo-pty-test"

	cmd := exec.Command("/bin/sh", "-c", "echo "+marker)
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 24, Cols: 80})
	if err != nil {
		t.Fatalf("pty.StartWithSize: %v", err)
	}
	t.Cleanup(func() { _ = ptmx.Close() })

	done := make(chan string, 1)
	go func() {
		out, _ := io.ReadAll(ptmx)
		done <- string(out)
	}()

	select {
	case out := <-done:
		if !strings.Contains(out, marker) {
			t.Errorf("PTY output %q does not contain marker %q", out, marker)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out reading PTY output")
	}
}
