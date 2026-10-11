package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/omartelo/lich/native/lichclient"
)

func TestCardWearsTheLiveAgentUntilItLeaves(t *testing.T) {
	m := newTestModel(nil)
	steps := []struct {
		name string
		data any
		want string
	}{
		{"session-agent", agentEvent{ID: "s1", Agent: "claude"}, "claude"},
		{"session-agent", agentEvent{ID: "s1", Agent: ""}, ""},
		{"session-agent", agentEvent{ID: "s1", Agent: "codex"}, "codex"},
		{"session-agent", agentEvent{ID: "s1", Agent: "from-a-newer-lich"}, ""},
		{"session-agent", agentEvent{ID: "s1", Agent: "claude"}, "claude"},
		{statusEventName, map[string]string{"id": "s1", "state": "busy"}, "claude"},
		{statusEventName, map[string]string{"id": "s1", "state": "idle"}, ""},
	}
	for i, s := range steps {
		send(m, s.name, s.data)
		if got := cardOf(t, m).agent; got != s.want {
			t.Fatalf("step %d (%s %v): agent %q, want %q", i, s.name, s.data, got, s.want)
		}
	}
}

func TestCardMarksASandboxedSpawn(t *testing.T) {
	m := newTestModel(nil)
	send(m, "session-sandbox", sandboxEvent{ID: "s1", Confined: true})
	if !cardOf(t, m).sandbox.Confined {
		t.Fatal("confined spawn: card not marked")
	}
	send(m, "session-sandbox", sandboxEvent{ID: "s1", Confined: false})
	if cardOf(t, m).sandbox.Confined {
		t.Fatal("unconfined respawn: card still marked")
	}
}

func TestPinFlipsTheCard(t *testing.T) {
	m := newTestModel(nil)
	if !m.setPinned("s1", true) || !cardOf(t, m).Pinned {
		t.Fatal("pin: card not pinned")
	}
	if m.setPinned("gone", true) {
		t.Fatal("pin of an unknown id reported true")
	}
}

// fakeUnread serves LoadState with one unread session and records every
// SetSessionUnread call.
func fakeUnread(t *testing.T, calls chan<- []any) *lichclient.Client {
	t.Helper()
	projects := []lichclient.Project{{ID: "p", Path: checkout, Sessions: []lichclient.Session{{ID: "s1", Unread: true}, {ID: "s2"}}}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rpc/store.LoadState":
			_ = json.NewEncoder(w).Encode(projects)
		case "/rpc/store.GetSetting":
			_ = json.NewEncoder(w).Encode("")
		case "/rpc/store.SetSessionUnread":
			var args []any
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &args)
			calls <- args
			_ = json.NewEncoder(w).Encode(nil)
		default:
			t.Errorf("unexpected call %s", r.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	return lichclient.New(lichclient.Runtime{Port: port, Token: "tok"})
}

func unreadOf(m *model, id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.status.unread(id)
}

func TestUnreadTurnIsReadOnlyWhileSomeoneWatches(t *testing.T) {
	ctx := context.Background()
	calls := make(chan []any, 4)
	m := newModel(fakeUnread(t, calls), func() {})
	if err := m.reload(ctx); err != nil {
		t.Fatal(err)
	}
	if !unreadOf(m, "s1") {
		t.Fatal("after LoadState: s1 not unread, want the stored mark restored")
	}

	m.setActive(ctx, "s1")
	if !unreadOf(m, "s1") {
		t.Fatal("put on screen in an unfocused window: read, want still unread")
	}

	m.setFocused(ctx, true)
	if unreadOf(m, "s1") {
		t.Fatal("window focused on s1: still unread")
	}
	select {
	case args := <-calls:
		if len(args) != 2 || args[0] != "s1" || args[1] != false {
			t.Fatalf("SetSessionUnread%v, want [s1 false]", args)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the backend's unread mark was never taken down")
	}
}

func TestRelayMarkFollowsTheRequest(t *testing.T) {
	m := newTestModel(nil)
	steps := []struct {
		name string
		data any
		want sessionRelay
	}{
		{"session-relay", sessionRelay{ID: "s1", Peer: "Claude 2", Direction: relayOut}, sessionRelay{ID: "s1", Peer: "Claude 2", Direction: relayOut}},
		{"session-relay", sessionRelay{ID: "s1"}, sessionRelay{}},
		{"session-relay", sessionRelay{ID: "s1", Direction: relayIn}, sessionRelay{ID: "s1", Direction: relayIn}},
		{"session-relay", sessionRelay{ID: "s1", Direction: "sideways"}, sessionRelay{}},
		{"session-relay", sessionRelay{ID: "s1", Direction: relayIn}, sessionRelay{ID: "s1", Direction: relayIn}},
		{statusEventName, map[string]string{"id": "s1", "state": "idle"}, sessionRelay{}},
	}
	for i, s := range steps {
		send(m, s.name, s.data)
		if got := cardOf(t, m).relay; got != s.want {
			t.Fatalf("step %d (%s %v): relay %+v, want %+v", i, s.name, s.data, got, s.want)
		}
	}
}
