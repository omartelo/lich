package terminal

import (
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty"
)

func TestDefaultShell(t *testing.T) {
	original, had := os.LookupEnv("SHELL")
	t.Cleanup(func() {
		if had {
			os.Setenv("SHELL", original)
		} else {
			os.Unsetenv("SHELL")
		}
	})

	os.Setenv("SHELL", "/custom/shell")
	if got := defaultShell(); got != "/custom/shell" {
		t.Errorf("defaultShell() with $SHELL set = %q, want %q", got, "/custom/shell")
	}

	os.Unsetenv("SHELL")
	if got := defaultShell(); got != "/bin/sh" {
		t.Errorf("defaultShell() with $SHELL unset = %q, want %q", got, "/bin/sh")
	}
}

// TestPTYEcho proves the core assumption of the service: the detected shell
// spawns under a PTY and its output is readable. If creack/pty or the shell
// setup breaks, this fails.
func TestPTYEcho(t *testing.T) {
	const marker = "skipo-pty-test"

	cmd := exec.Command(defaultShell(), "-c", "echo "+marker)
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
