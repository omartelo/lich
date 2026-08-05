package events

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestEmitDropsWithoutClient(t *testing.T) {
	hub := New()
	hub.Emit("session-touched", nil) // must not panic
}

func TestEmitDeliversToConnectedClient(t *testing.T) {
	hub := New()
	server := httptest.NewServer(hub)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	url := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.CloseNow()

	waitForAttach(t, hub)
	hub.Emit("session-title", map[string]string{"id": "s1", "label": "x"})

	_, payload, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var env Envelope
	if err := json.Unmarshal(payload, &env); err != nil || env.Name != "session-title" {
		t.Fatalf("envelope: %s (%v)", payload, err)
	}
}

func TestEmitDropsDeadClient(t *testing.T) {
	hub := New()
	server := httptest.NewServer(hub)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	url := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	waitForAttach(t, hub)
	_ = conn.CloseNow()

	// The write to a dead peer may take one emit to surface; the hub must
	// eventually detach the connection instead of retrying it forever.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		hub.Emit("session-touched", map[string]string{"id": "s1"})
		hub.mu.Lock()
		detached := hub.conn == nil
		hub.mu.Unlock()
		if detached {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("dead client never detached")
}

// TestReplacedClientDoesNotSilenceHub models a page reload: a second /events
// socket opens, the hub replaces the first, and the first's read loop then
// drops the connection it was reading. The live client must keep receiving —
// forgetting the replaced connection must not forget the current one. A hub
// that goes silent here never closes the live socket either, so the window,
// which only reconnects on close, stays dark until the page is reloaded again.
func TestReplacedClientDoesNotSilenceHub(t *testing.T) {
	hub := New()
	server := httptest.NewServer(hub)
	defer server.Close()

	swapped, cancelSwap := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelSwap()
	url := "ws" + strings.TrimPrefix(server.URL, "http")

	first, _, err := websocket.Dial(swapped, url, nil)
	if err != nil {
		t.Fatalf("dial first: %v", err)
	}
	defer first.CloseNow()
	waitForAttach(t, hub)
	hub.mu.Lock()
	replaced := hub.conn
	hub.mu.Unlock()

	second, _, err := websocket.Dial(swapped, url, nil)
	if err != nil {
		t.Fatalf("dial second: %v", err)
	}
	defer second.CloseNow()

	// attach swaps the hub's connection before it closes the one it replaced,
	// so the first client's read failing proves the second one is already the
	// hub's client. Polling hub.conn would be the worse signal: without the
	// identity guard the read loop nils it out, and the wait would fail on its
	// own before the assertion below ever ran.
	if _, _, err := first.Read(swapped); err == nil {
		t.Fatal("the hub never closed the replaced client")
	}

	// The replaced connection's read loop calls drop on its own the moment it
	// notices attach closed it; calling drop here asserts on that outcome
	// instead of racing the goroutine for it. Under the identity guard the
	// second call is the no-op the first one already was.
	hub.drop(replaced)

	hub.Emit("session-title", map[string]string{"id": "s1", "label": "x"})

	// Its own deadline: sharing the one above would let a slow swap eat this
	// wait's budget and report a silent hub that is merely late.
	delivered, cancelDelivery := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelDelivery()
	_, payload, err := second.Read(delivered)
	if err != nil {
		t.Fatalf("live client received nothing after the replaced one was dropped: %v", err)
	}
	var env Envelope
	if err := json.Unmarshal(payload, &env); err != nil || env.Name != "session-title" {
		t.Fatalf("envelope: %s (%v)", payload, err)
	}
}

func waitForAttach(t *testing.T, hub *Hub) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		hub.mu.Lock()
		attached := hub.conn != nil
		hub.mu.Unlock()
		if attached {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("client never attached")
}
