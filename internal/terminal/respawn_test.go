// These tests drive real PTYs and a real /events websocket client, so they are
// Unix-only like the rest of the service's spawn tests (terminal_test.go).
//go:build !windows

package terminal

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/omartelo/lich/internal/events"
)

// reapSettle bounds how long a test watches a dead PTY's reap after that PTY's
// outbox has closed. All the reap has left to do by then is take a mutex, look
// up one map key and emit over a loopback socket, so the window is orders of
// magnitude wider than it needs — wide enough that a slow runner cannot turn
// these assertions into a coin flip.
const reapSettle = 250 * time.Millisecond

// respawn runs the lifecycle both tests need — start a session, close it,
// start it again under the same id — and returns the session whose PTY died,
// once its stream goroutine has delivered its last frame. From there the reap
// is all that goroutine has left to do.
func respawn(t *testing.T, svc *Service, id string) *session {
	t.Helper()
	if err := svc.Start(id, "p1", t.TempDir(), "claude", "", "", false, false, 80, 24); err != nil {
		t.Fatalf("first start: %v", err)
	}
	svc.mu.Lock()
	dead := svc.sessions[id]
	svc.mu.Unlock()
	if dead == nil {
		t.Fatal("the first start registered no session")
	}

	if err := svc.Close(id); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := svc.Start(id, "p1", t.TempDir(), "claude", "", "", false, false, 80, 24); err != nil {
		t.Fatalf("second start: %v", err)
	}

	<-dead.outbox.done
	return dead
}

// TestRespawnedSessionIsNotToldItExited proves the exit event belongs to the
// PTY that died, not to whichever session holds its id by the time the reap
// runs. The frontend writes "[process exited]" into the terminal on that event,
// so one landing on a session respawned under the same id lies about a live
// shell.
func TestRespawnedSessionIsNotToldItExited(t *testing.T) {
	hub, rec := newProbeHub(t)
	svc := New(stubBins{bin: echoBin(t)}, nil, hub)
	t.Cleanup(func() { _ = svc.Close("s1") })

	respawn(t, svc, "s1")

	deadline := time.Now().Add(reapSettle)
	for time.Now().Before(deadline) {
		if slices.Contains(rec.snapshot(), exitEventPrefix+"s1") {
			t.Fatal("the dead PTY's reap emitted an exit event for the respawned session")
		}
		time.Sleep(time.Millisecond)
	}
}

// TestRespawnedSessionKeepsItsPTY proves the other half of the same reap: it
// evicts only its own PTY, so after Close+Start under one id that id still
// resolves to the new PTY and input still reaches it.
func TestRespawnedSessionKeepsItsPTY(t *testing.T) {
	svc := New(stubBins{bin: echoBin(t)}, nil, events.New())
	t.Cleanup(func() { _ = svc.Close("s1") })

	respawn(t, svc, "s1")

	deadline := time.Now().Add(reapSettle)
	for time.Now().Before(deadline) {
		if svc.ptyOf("s1") == nil {
			t.Fatal("the dead PTY's reap evicted the respawned session")
		}
		time.Sleep(time.Millisecond)
	}

	const marker = "lich-respawn-marker"
	if err := svc.Write("s1", marker+"\n"); err != nil {
		t.Fatalf("write: %v", err)
	}
	waitFor(t, func() bool {
		return strings.Contains(replayOf(t, svc, "s1"), marker)
	}, "input to reach the respawned PTY")
}

// TestExitEventCarriesTheExitCode proves the status the child exited with
// travels all the way to the window: the seam's Wait, the payload and the event.
// The card reads it as the reason the session ended, and only a real spawn shows
// whether the number survives the trip.
func TestExitEventCarriesTheExitCode(t *testing.T) {
	hub, rec := newProbeHub(t)
	svc := New(stubBins{bin: exitBin(t, 3)}, nil, hub)
	t.Cleanup(func() { _ = svc.Close("s1") })

	if err := svc.Start("s1", "p1", t.TempDir(), "claude", "", "", false, false, 80, 24); err != nil {
		t.Fatalf("start: %v", err)
	}

	name := exitEventPrefix + "s1"
	waitFor(t, func() bool {
		_, ok := rec.payloadOf(name)
		return ok
	}, "the exit event")

	payload, _ := rec.payloadOf(name)
	fields, ok := payload.(map[string]any)
	if !ok {
		t.Fatalf("exit payload is %T, want a JSON object", payload)
	}
	if got := fields["code"]; got != float64(3) {
		t.Errorf("code = %v, want 3", got)
	}
}

// TestExitEventOmitsAnUnobservedStatus is the other half of that contract, on
// the path that produces it: a child killed by a signal left no exit status, so
// the event has to say nothing rather than report the 0 of a clean exit.
func TestExitEventOmitsAnUnobservedStatus(t *testing.T) {
	hub, rec := newProbeHub(t)
	// echoBin execs cat, so the pid below is the process holding the PTY —
	// a script that forked one would leave the slave open and never reach EOF.
	svc := New(stubBins{bin: echoBin(t)}, nil, hub)
	t.Cleanup(func() { _ = svc.Close("s1") })

	if err := svc.Start("s1", "p1", t.TempDir(), "claude", "", "", false, false, 80, 24); err != nil {
		t.Fatalf("start: %v", err)
	}
	// Killed behind the service's back: Close would evict the session first and
	// the reap would then emit nothing at all (see stream).
	if err := syscall.Kill(svc.ptyOf("s1").Pid(), syscall.SIGKILL); err != nil {
		t.Fatalf("kill child: %v", err)
	}

	name := exitEventPrefix + "s1"
	waitFor(t, func() bool {
		_, ok := rec.payloadOf(name)
		return ok
	}, "the exit event")

	payload, _ := rec.payloadOf(name)
	fields, _ := payload.(map[string]any)
	if code, ok := fields["code"]; ok {
		t.Errorf("code = %v, want no code at all", code)
	}
}

// exitBin writes a script that exits with code straight away, so a test can read
// the status a session reports when its process is gone.
func exitBin(t *testing.T, code int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fake-claude")
	script := fmt.Sprintf("#!/bin/sh\nexit %d\n", code)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write stub bin: %v", err)
	}
	return path
}

// replayOf returns a session's replay buffer decoded back to raw output.
func replayOf(t *testing.T, svc *Service, id string) string {
	t.Helper()
	encoded, err := svc.Replay(id)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("decode replay: %v", err)
	}
	return string(raw)
}
