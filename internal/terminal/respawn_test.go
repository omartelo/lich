// These tests drive real PTYs and a real /events websocket client, so they are
// Unix-only like the rest of the service's spawn tests (terminal_test.go).
//go:build !windows

package terminal

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/omartelo/lich/internal/events"
)

// reapSettle bounds how long a test watches a dead PTY's reap after that PTY's
// outbox has closed. All the reap has left to do by then is take a mutex, look
// up one map key and emit over a loopback socket, so the window is orders of
// magnitude wider than it needs — wide enough that a slow runner cannot turn
// these assertions into a coin flip.
const reapSettle = 250 * time.Millisecond

// probeReadyEvent is the sentinel newProbeHub emits until its client is
// attached. The hub drops events while nobody is connected, so one coming back
// is the only proof, from outside the events package, that the socket is live.
const probeReadyEvent = "probe-ready"

// eventRecorder collects, in order, the name of every event the hub pushed to
// its client.
type eventRecorder struct {
	mu    sync.Mutex
	names []string
}

func (r *eventRecorder) add(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.names = append(r.names, name)
}

func (r *eventRecorder) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.names)
}

func (r *eventRecorder) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.names = nil
}

// newProbeHub returns a hub with a real websocket client attached plus a
// recorder of every event name that client received, so a test can assert on
// what the service actually pushed to the window. The record starts empty: the
// socket is FIFO, so one last sentinel coming back proves every straggler from
// the attach loop was already recorded when the reset ran.
func newProbeHub(t *testing.T) (*events.Hub, *eventRecorder) {
	t.Helper()
	hub := events.New()
	server := httptest.NewServer(hub)
	t.Cleanup(server.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatalf("dial probe hub: %v", err)
	}
	t.Cleanup(func() { _ = conn.CloseNow() })

	rec := &eventRecorder{}
	go func() {
		for {
			_, payload, err := conn.Read(context.Background())
			if err != nil {
				return
			}
			var envelope events.Envelope
			if err := json.Unmarshal(payload, &envelope); err != nil {
				return
			}
			rec.add(envelope.Name)
		}
	}()

	deadline := time.Now().Add(5 * time.Second)
	for len(rec.snapshot()) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("probe hub client never attached")
		}
		hub.Emit(probeReadyEvent, nil)
		time.Sleep(time.Millisecond)
	}
	settled := len(rec.snapshot())
	hub.Emit(probeReadyEvent, nil)
	waitFor(t, func() bool { return len(rec.snapshot()) > settled }, "the probe hub to settle")
	rec.reset()
	return hub, rec
}

// waitFor polls cond until it holds, failing the test if it never does. Every
// wait in these tests is for something that must happen, never for a duration.
func waitFor(t *testing.T, cond func() bool, what string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// respawn runs the lifecycle both tests need — start a session, close it,
// start it again under the same id — and returns the session whose PTY died,
// once its stream goroutine has delivered its last frame. From there the reap
// is all that goroutine has left to do.
func respawn(t *testing.T, svc *Service, id string) *session {
	t.Helper()
	if err := svc.Start(id, "p1", t.TempDir(), "claude", "", false, 80, 24); err != nil {
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
	if err := svc.Start(id, "p1", t.TempDir(), "claude", "", false, 80, 24); err != nil {
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
	svc := New(stubBins{bin: "/bin/cat"}, nil, hub)
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
	svc := New(stubBins{bin: "/bin/cat"}, nil, events.New())
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
