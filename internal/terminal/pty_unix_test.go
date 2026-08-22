//go:build !windows

package terminal

import (
	"bytes"
	"io"
	"testing"
	"time"
)

// startTrapped starts a shell that installs trap for SIGTERM and then idles,
// and returns once that shell says the trap is armed — a hang-up delivered
// before it is armed would be answered by the default disposition, and the
// test would be measuring the kernel instead of the shell.
func startTrapped(t *testing.T, trap string) ptyHandle {
	t.Helper()
	p, err := startPTY(ptySpec{
		bin:  "/bin/sh",
		args: []string{"-c", "trap '" + trap + "' TERM; echo armed; while :; do sleep 0.05; done"},
		cols: 80,
		rows: 24,
	})
	if err != nil {
		t.Fatalf("startPTY: %v", err)
	}
	t.Cleanup(func() { _ = p.Close() })

	armed := make(chan struct{})
	go func() {
		var seen bytes.Buffer
		buf := make([]byte, 64)
		for {
			n, err := p.Read(buf)
			seen.Write(buf[:n])
			if bytes.Contains(seen.Bytes(), []byte("armed")) {
				close(armed)
				_, _ = io.Copy(io.Discard, p)
				return
			}
			if err != nil {
				return
			}
		}
	}()
	select {
	case <-armed:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for the shell to arm its SIGTERM trap")
	}
	return p
}

// TestCloseHangsUpBeforeKilling proves a session close reaches the child as
// SIGTERM: the shell exits through its own handler with a status of its own
// choosing, which a kill could never produce. That signal is what lets an agent
// run its exit path — Claude Code counts a session killed during its first ten
// seconds as a failed fullscreen start (see closeGrace).
func TestCloseHangsUpBeforeKilling(t *testing.T) {
	p := startTrapped(t, "exit 7")

	if err := p.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	code, _ := p.Wait()
	if code != 7 {
		t.Errorf("exit status = %d, want the trap's 7 (a kill leaves %d)", code, noExitStatus)
	}
}

// TestCloseKillsAChildThatIgnoresTheHangUp proves the grace period is a grace
// period and not a promise: a child that ignores SIGTERM is still gone when
// Close returns, the way it was before the signal was ever sent.
func TestCloseKillsAChildThatIgnoresTheHangUp(t *testing.T) {
	p := startTrapped(t, "")

	start := time.Now()
	if err := p.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if waited := time.Since(start); waited < closeGrace {
		t.Errorf("Close returned after %v, want it to wait out the %v grace", waited, closeGrace)
	}

	done := make(chan int, 1)
	go func() { code, _ := p.Wait(); done <- code }()
	select {
	case code := <-done:
		if code != noExitStatus {
			t.Errorf("exit status = %d, want %d for a killed child", code, noExitStatus)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("child survived Close")
	}
}
