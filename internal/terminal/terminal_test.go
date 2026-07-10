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
	if err := svc.SetVisible("ghost", true); err != nil {
		t.Errorf("SetVisible unknown = %v, want nil", err)
	}
}

// TestSetVisibleReachesCoalescer proves the service routes visibility flips to
// the session's coalescer: output buffered while hidden is flushed when the
// session is made visible.
func TestSetVisibleReachesCoalescer(t *testing.T) {
	emit, emits := captureEmit(1)
	out := newCoalescer(emit, time.Hour, time.Hour)
	out.SetVisible(false)
	out.Write([]byte("pending"))

	svc := New(stubBins{})
	sess := spawnSession(t)
	sess.out = out
	svc.sessions["s1"] = sess

	if err := svc.SetVisible("s1", true); err != nil {
		t.Fatalf("SetVisible = %v, want nil", err)
	}
	select {
	case got := <-emits:
		if string(got) != "pending" {
			t.Errorf("flushed %q, want %q", got, "pending")
		}
	default:
		t.Error("SetVisible(true) did not flush the coalescer")
	}
}

// spawnSession starts /bin/cat under a PTY and returns a live session, keeping
// the process off the Wails event singleton that stream() needs.
func spawnSession(t *testing.T) *session {
	t.Helper()
	cmd := exec.Command("/bin/cat")
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 24, Cols: 80})
	if err != nil {
		t.Fatalf("pty.StartWithSize: %v", err)
	}
	t.Cleanup(func() {
		_ = ptmx.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	})
	return &session{ptmx: ptmx, cmd: cmd}
}

// TestWriteResizeCloseOnLiveSession drives a real session end to end: input is
// written, the window is resized and Close reaps the shell and drops it.
func TestWriteResizeCloseOnLiveSession(t *testing.T) {
	svc := New(stubBins{})
	svc.sessions["s1"] = spawnSession(t)

	if err := svc.Write("s1", "hello"); err != nil {
		t.Errorf("Write = %v, want nil", err)
	}
	if err := svc.Resize("s1", 100, 40); err != nil {
		t.Errorf("Resize = %v, want nil", err)
	}
	if err := svc.Close("s1"); err != nil {
		t.Errorf("Close = %v, want nil", err)
	}
	if svc.ptmxOf("s1") != nil {
		t.Error("session still present after Close")
	}
}

// TestStartIsNoopWhenAlreadyRunning proves Start returns without spawning a
// second shell for a session ID that is already tracked.
func TestStartIsNoopWhenAlreadyRunning(t *testing.T) {
	svc := New(stubBins{})
	sess := spawnSession(t)
	svc.sessions["s1"] = sess

	if err := svc.Start("s1", "p1", "", "", 80, 24); err != nil {
		t.Errorf("Start(running) = %v, want nil", err)
	}
	if svc.sessions["s1"] != sess {
		t.Error("Start replaced the running session")
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

// TestResolveCommand proves kind selects between the user's shell and the
// Claude Code binary, with fallbacks when either source is empty.
func TestResolveCommand(t *testing.T) {
	cases := []struct {
		name, kind, bin, shell, want string
	}{
		{"claude default", "", "", "/bin/zsh", defaultBin},
		{"claude custom bin", "claude", "/opt/claude.sh", "/bin/zsh", "/opt/claude.sh"},
		{"shell from env", KindShell, "/opt/claude.sh", "/bin/zsh", "/bin/zsh"},
		{"shell fallback", KindShell, "", "", defaultShell},
	}
	for _, tc := range cases {
		if got := resolveCommand(tc.kind, tc.bin, tc.shell); got != tc.want {
			t.Errorf("%s: resolveCommand(%q, %q, %q) = %q, want %q",
				tc.name, tc.kind, tc.bin, tc.shell, got, tc.want)
		}
	}
}

// TestPTYEcho proves the core assumption of the service: a process spawns under
// a PTY and its output is readable. If creack/pty breaks, this fails.
func TestPTYEcho(t *testing.T) {
	const marker = "lich-pty-test"

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
