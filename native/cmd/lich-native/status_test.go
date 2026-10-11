package main

import (
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/omartelo/lich/native/lichclient"
)

var t0 = time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)

func statusEv(t *testing.T, p statusEvent) lichclient.Event {
	t.Helper()
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	return lichclient.Event{Name: statusEventName, Data: data}
}

// step is one report, landing secs seconds after t0.
type step struct {
	secs int
	ev   statusEvent
}

func replay(t *testing.T, s *statusStore, steps []step) []bool {
	t.Helper()
	var changed []bool
	for _, st := range steps {
		c, err := s.apply(statusEv(t, st.ev), t0.Add(time.Duration(st.secs)*time.Second))
		if err != nil {
			t.Fatal(err)
		}
		changed = append(changed, c)
	}
	return changed
}

func TestStatusStoreSequences(t *testing.T) {
	cases := []struct {
		name       string
		steps      []step
		wantStatus string
		wantReason string
		wantTool   *sessionTool
		wantUnread bool
		wantAge    time.Duration
		wantAged   bool
		changed    []bool
	}{
		{
			name:       "busy runs with an age from its transition",
			steps:      []step{{0, statusEvent{ID: "a", State: "busy"}}, {5, statusEvent{ID: "a", State: "busy"}}},
			wantStatus: statusBusy, wantAge: 10 * time.Second, wantAged: true,
			changed: []bool{true, false},
		},
		{
			name:       "done is unread and has no age",
			steps:      []step{{0, statusEvent{ID: "a", State: "busy"}}, {3, statusEvent{ID: "a", State: "done"}}},
			wantStatus: statusDone, wantUnread: true,
			changed: []bool{true, true},
		},
		{
			name:       "idle clears the status",
			steps:      []step{{0, statusEvent{ID: "a", State: "busy"}}, {1, statusEvent{ID: "a", State: "idle"}}},
			wantStatus: statusNone,
			changed:    []bool{true, true},
		},
		{
			name:       "interrupted clears the status",
			steps:      []step{{0, statusEvent{ID: "a", State: "waiting", Reason: "perm"}}, {1, statusEvent{ID: "a", State: "interrupted"}}},
			wantStatus: statusNone,
			changed:    []bool{true, true},
		},
		{
			name:       "an unknown state clears the status",
			steps:      []step{{0, statusEvent{ID: "a", State: "done"}}, {1, statusEvent{ID: "a", State: "dreaming"}}},
			wantStatus: statusNone,
			changed:    []bool{true, true},
		},
		{
			name:       "first report is news even when it maps to no status",
			steps:      []step{{0, statusEvent{ID: "a", State: "idle"}}, {1, statusEvent{ID: "a", State: "idle"}}},
			wantStatus: statusNone,
			changed:    []bool{true, false},
		},
		{
			name: "a new reason in the same waiting is news, age keeps its start",
			steps: []step{
				{0, statusEvent{ID: "a", State: "waiting", Reason: "Bash"}},
				{4, statusEvent{ID: "a", State: "waiting", Reason: "Edit"}},
				{6, statusEvent{ID: "a", State: "waiting", Reason: "Edit"}},
			},
			wantStatus: statusWaiting, wantReason: "Edit", wantAge: 10 * time.Second, wantAged: true,
			changed: []bool{true, true, false},
		},
		{
			name:       "reason rides on waiting alone",
			steps:      []step{{0, statusEvent{ID: "a", State: "busy", Reason: "stray"}}},
			wantStatus: statusBusy, wantAge: 10 * time.Second, wantAged: true,
			changed: []bool{true},
		},
		{
			name:       "leaving waiting clears the reason",
			steps:      []step{{0, statusEvent{ID: "a", State: "waiting", Reason: "perm"}}, {2, statusEvent{ID: "a", State: "busy"}}},
			wantStatus: statusBusy, wantAge: 8 * time.Second, wantAged: true,
			changed: []bool{true, true},
		},
		{
			name: "busy naming a tool sets it; busy naming none keeps it",
			steps: []step{
				{0, statusEvent{ID: "a", State: "busy", Tool: "Bash", Detail: "ls"}},
				{1, statusEvent{ID: "a", State: "busy"}},
			},
			wantStatus: statusBusy, wantTool: &sessionTool{Name: "Bash", Detail: "ls"}, wantAge: 10 * time.Second, wantAged: true,
			changed: []bool{true, false},
		},
		{
			name: "a repeat busy carrying a new tool is news",
			steps: []step{
				{0, statusEvent{ID: "a", State: "busy", Tool: "Bash", Detail: "ls"}},
				{1, statusEvent{ID: "a", State: "busy", Tool: "Read", Detail: "go.mod"}},
			},
			wantStatus: statusBusy, wantTool: &sessionTool{Name: "Read", Detail: "go.mod"}, wantAge: 10 * time.Second, wantAged: true,
			changed: []bool{true, true},
		},
		{
			name: "any state but busy clears the tool, waiting included",
			steps: []step{
				{0, statusEvent{ID: "a", State: "busy", Tool: "Bash"}},
				{1, statusEvent{ID: "a", State: "waiting", Tool: "Bash"}},
			},
			wantStatus: statusWaiting, wantAge: 9 * time.Second, wantAged: true,
			changed: []bool{true, true},
		},
		{
			name: "idle clears the tool",
			steps: []step{
				{0, statusEvent{ID: "a", State: "busy", Tool: "Bash"}},
				{1, statusEvent{ID: "a", State: "idle"}},
			},
			wantStatus: statusNone,
			changed:    []bool{true, true},
		},
		{
			name:       "compacting runs with an age",
			steps:      []step{{0, statusEvent{ID: "a", State: "compacting"}}},
			wantStatus: statusCompacting, wantAge: 10 * time.Second, wantAged: true,
			changed: []bool{true},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newStatusStore()
			changed := replay(t, s, tc.steps)
			if !slices.Equal(changed, tc.changed) {
				t.Errorf("changed = %v, want %v", changed, tc.changed)
			}
			if got := s.status("a"); got != tc.wantStatus {
				t.Errorf("status = %q, want %q", got, tc.wantStatus)
			}
			if got := s.reason("a"); got != tc.wantReason {
				t.Errorf("reason = %q, want %q", got, tc.wantReason)
			}
			tool, ok := s.tool("a")
			if (tc.wantTool != nil) != ok || (ok && tool != *tc.wantTool) {
				t.Errorf("tool = %v %v, want %v", tool, ok, tc.wantTool)
			}
			if got := s.unread("a"); got != tc.wantUnread {
				t.Errorf("unread = %v, want %v", got, tc.wantUnread)
			}
			age, aged := s.age("a", t0.Add(10*time.Second))
			if age != tc.wantAge || aged != tc.wantAged {
				t.Errorf("age = %v %v, want %v %v", age, aged, tc.wantAge, tc.wantAged)
			}
			if !s.reported("a") {
				t.Error("reported = false after a report")
			}
		})
	}
}

func TestStatusStoreUnknownSession(t *testing.T) {
	s := newStatusStore()
	if s.status("x") != statusNone || s.reason("x") != "" || s.reported("x") || s.unread("x") {
		t.Error("an unknown session answered as if it had reported")
	}
	if _, ok := s.tool("x"); ok {
		t.Error("an unknown session has a tool")
	}
	if _, ok := s.age("x", t0); ok {
		t.Error("an unknown session has an age")
	}
	if s.markSeen("x") || s.dismiss("x") {
		t.Error("reading an unknown session asked for its mark to come down")
	}
}

func TestStatusStoreAgeClampsBackwardsClock(t *testing.T) {
	s := newStatusStore()
	replay(t, s, []step{{10, statusEvent{ID: "a", State: "busy"}}})
	if age, ok := s.age("a", t0); age != 0 || !ok {
		t.Errorf("age = %v %v, want 0 true", age, ok)
	}
}

func TestStatusStoreApplyErrors(t *testing.T) {
	s := newStatusStore()
	if changed, err := s.apply(lichclient.Event{Name: "session-cwd", Data: []byte(`{"id":"a"}`)}, t0); changed || err != nil {
		t.Errorf("other event = %v %v, want ignored", changed, err)
	}
	if _, err := s.apply(lichclient.Event{Name: statusEventName, Data: []byte(`{"id":"a","state":1}`)}, t0); err == nil {
		t.Error("a non-string state decoded")
	}
	if _, err := s.apply(lichclient.Event{Name: statusEventName, Data: []byte(`{"state":"busy"}`)}, t0); err == nil {
		t.Error("a payload with no id was accepted")
	}
	if s.reported("") {
		t.Error("a rejected payload left an entry")
	}
}

func TestStatusStoreSeenAndDismiss(t *testing.T) {
	s := newStatusStore()
	replay(t, s, []step{
		{0, statusEvent{ID: "done", State: "done"}},
		{0, statusEvent{ID: "wait", State: "waiting"}},
		{0, statusEvent{ID: "busy", State: "busy"}},
	})
	want := []pendingStatus{{"done", statusDone}, {"wait", statusWaiting}}
	if got := s.pending(); !slices.Equal(got, want) {
		t.Fatalf("pending = %v, want %v", got, want)
	}
	if !s.markSeen("done") {
		t.Error("reading a finished turn did not ask for its mark to come down")
	}
	if s.markSeen("done") {
		t.Error("a second read asked again")
	}
	if s.unread("done") {
		t.Error("a read turn is still unread")
	}
	if s.markSeen("busy") {
		t.Error("reading a busy session asked for a mark to come down")
	}
	if s.dismiss("wait") {
		t.Error("dismissing a waiting session asked for a mark to come down")
	}
	if got := s.pending(); len(got) != 0 {
		t.Errorf("pending = %v, want empty", got)
	}
	if got := s.projectStatus([]string{"done", "wait"}); got != statusWaiting {
		t.Errorf("a dismissed waiting stopped badging: %q", got)
	}
	replay(t, s, []step{{1, statusEvent{ID: "wait", State: "waiting", Reason: "again"}}})
	if got := s.pending(); !slices.Equal(got, []pendingStatus{{"wait", statusWaiting}}) {
		t.Errorf("a fresh report did not undo the dismiss: %v", got)
	}
	replay(t, s, []step{{2, statusEvent{ID: "done", State: "busy"}}, {3, statusEvent{ID: "done", State: "done"}}})
	if !s.unread("done") {
		t.Error("a fresh finished turn is not unread")
	}
}

func TestStatusStoreDismissDoneReads(t *testing.T) {
	s := newStatusStore()
	replay(t, s, []step{{0, statusEvent{ID: "a", State: "done"}}})
	if !s.dismiss("a") {
		t.Error("dismissing a finished turn did not read it")
	}
	if s.unread("a") {
		t.Error("a dismissed finished turn is still unread")
	}
}

func TestStatusStoreRestoreUnread(t *testing.T) {
	s := newStatusStore()
	replay(t, s, []step{{0, statusEvent{ID: "live", State: "busy"}}})
	s.restoreUnread([]string{"live", "back"})
	if s.status("live") != statusBusy {
		t.Error("restore overwrote a session a report already spoke for")
	}
	if s.status("back") != statusDone || !s.unread("back") || !s.reported("back") {
		t.Error("restore did not seed an unread finished turn")
	}
	if got := s.pending(); !slices.Equal(got, []pendingStatus{{"back", statusDone}}) {
		t.Errorf("pending = %v", got)
	}
}

func TestStatusStoreProjectStatus(t *testing.T) {
	cases := []struct {
		name   string
		states map[string]string
		seen   []string
		want   string
	}{
		{"nothing", map[string]string{}, nil, statusNone},
		{"waiting wins", map[string]string{"a": "done", "b": "busy", "c": "waiting"}, nil, statusWaiting},
		{"busy over compacting", map[string]string{"a": "compacting", "b": "busy"}, nil, statusBusy},
		{"compacting over done", map[string]string{"a": "done", "b": "compacting"}, nil, statusCompacting},
		{"unread done", map[string]string{"a": "done"}, nil, statusDone},
		{"read done says nothing", map[string]string{"a": "done", "b": "idle"}, []string{"a"}, statusNone},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newStatusStore()
			var ids []string
			for id, state := range tc.states {
				replay(t, s, []step{{0, statusEvent{ID: id, State: state}}})
				ids = append(ids, id)
			}
			for _, id := range tc.seen {
				s.markSeen(id)
			}
			if got := s.projectStatus(append(ids, "unknown")); got != tc.want {
				t.Errorf("projectStatus = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestStatusStoreRunning(t *testing.T) {
	s := newStatusStore()
	replay(t, s, []step{
		{0, statusEvent{ID: "b", State: "busy"}},
		{0, statusEvent{ID: "w", State: "waiting"}},
		{0, statusEvent{ID: "c", State: "compacting"}},
		{0, statusEvent{ID: "d", State: "done"}},
		{0, statusEvent{ID: "i", State: "idle"}},
	})
	s.markSeen("w")
	got := s.running([]string{"b", "w", "c", "d", "i", "unknown"})
	if want := []string{"b", "w", "c"}; !slices.Equal(got, want) {
		t.Errorf("running = %v, want %v", got, want)
	}
}
