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
