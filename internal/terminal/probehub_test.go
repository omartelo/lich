// A hub with a real /events client on the other end, so a test can assert on
// what the service actually pushed to the window rather than on the call it
// made. No PTY is involved, so this runs everywhere the suite does.

package terminal

import (
	"context"
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

// probeReadyEvent is the sentinel newProbeHub emits until its client is
// attached. The hub drops events while nobody is connected, so one coming back
// is the only proof, from outside the events package, that the socket is live.
const probeReadyEvent = "probe-ready"

// eventRecorder collects, in order, the name of every event the hub pushed to
// its client, plus the payload each name last carried.
type eventRecorder struct {
	mu       sync.Mutex
	names    []string
	payloads map[string]any
	// states is every session-status payload in the order it arrived, which is
	// what a test about the status stream asserts on: payloads keeps only the
	// last one per name, and a report that should never have been pushed is
	// exactly the one the next report would hide.
	states []statusEvent
}

func (r *eventRecorder) add(name string, data any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.names = append(r.names, name)
	if r.payloads == nil {
		r.payloads = make(map[string]any)
	}
	r.payloads[name] = data
	if name == statusEventName {
		r.states = append(r.states, decodeStatus(data))
	}
}

// decodeStatus reads a status payload back off the wire, where it arrived as
// the generic JSON object every envelope carries.
func decodeStatus(data any) statusEvent {
	fields, _ := data.(map[string]any)
	text := func(key string) string {
		value, _ := fields[key].(string)
		return value
	}
	return statusEvent{ID: text("id"), State: text("state"), Tool: text("tool"), Detail: text("detail")}
}

// payloadOf returns the payload of the last event pushed under this name, and
// whether any was pushed at all.
func (r *eventRecorder) payloadOf(name string) (any, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	data, ok := r.payloads[name]
	return data, ok
}

func (r *eventRecorder) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.names)
}

// statesOf returns the states pushed for one session, in order.
func (r *eventRecorder) statesOf(id string) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var states []string
	for _, event := range r.states {
		if event.ID == id {
			states = append(states, event.State)
		}
	}
	return states
}

func (r *eventRecorder) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.names = nil
	r.payloads = nil
	r.states = nil
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
			rec.add(envelope.Name, envelope.Data)
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
