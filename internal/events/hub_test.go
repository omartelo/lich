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

func TestEmitFallsBackWithoutClient(t *testing.T) {
	var gotName string
	var gotData any
	hub := New(func(name string, data any) {
		gotName = name
		gotData = data
	})
	hub.Emit("session-attention", map[string]string{"id": "s1"})
	if gotName != "session-attention" || gotData == nil {
		t.Fatalf("fallback not used: %q %v", gotName, gotData)
	}
}

func TestEmitDropsSilentlyWithoutFallback(t *testing.T) {
	hub := New(nil)
	hub.Emit("session-touched", nil) // must not panic
}

func TestEmitPrefersConnectedClient(t *testing.T) {
	fallbacks := 0
	hub := New(func(string, any) { fallbacks++ })
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
	if fallbacks != 0 {
		t.Fatalf("fallback used with client connected: %d", fallbacks)
	}
}

func TestEmitFallsBackAfterClientGone(t *testing.T) {
	fallbacks := 0
	hub := New(func(string, any) { fallbacks++ })
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

	// The write to a dead peer may take one emit to surface; both must land
	// in the fallback eventually.
	deadline := time.Now().Add(5 * time.Second)
	for fallbacks == 0 && time.Now().Before(deadline) {
		hub.Emit("session-touched", map[string]string{"id": "s1"})
		time.Sleep(10 * time.Millisecond)
	}
	if fallbacks == 0 {
		t.Fatal("never fell back after client closed")
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
