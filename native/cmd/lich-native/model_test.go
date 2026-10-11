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

	"github.com/omartelo/lich/native/lichclient"
)

const (
	checkout  = "/home/me/try/repo"
	elsewhere = "/home/me/try/other"
)

// fakeDiff serves project.Diff, answering branch "b:<path>" with one file per
// path it is asked about, and failing for notGit.
func fakeDiff(t *testing.T, notGit string) *lichclient.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var args []string
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &args); err != nil || r.URL.Path != "/rpc/project.Diff" {
			t.Errorf("unexpected call %s %s", r.URL.Path, body)
		}
		if args[0] == notGit {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "not a git repository"})
			return
		}
		_ = json.NewEncoder(w).Encode(lichclient.DiffStats{Files: 1, Added: 3, Deleted: 2, Branch: "b:" + args[0]})
	}))
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	return lichclient.New(lichclient.Runtime{Port: port, Token: "tok"})
}

func newTestModel(client *lichclient.Client) *model {
	m := newModel(client, func() {})
	m.projects = []lichclient.Project{{ID: "p", Path: checkout, Sessions: []lichclient.Session{{ID: "s1", Label: "Shell 1"}}}}
	return m
}

func cardOf(t *testing.T, m *model) sessionView {
	t.Helper()
	gs := m.groups()
	if len(gs) != 1 || len(gs[0].sessions) != 1 {
		t.Fatalf("got groups %+v, want one session", gs)
	}
	return gs[0].sessions[0]
}

func send(m *model, name string, data any) {
	raw, _ := json.Marshal(data)
	events := make(chan lichclient.Event, 1)
	events <- lichclient.Event{Name: name, Data: raw}
	close(events)
	m.follow(context.Background(), events)
}

func TestCardFollowsCwdAndReadsGitThere(t *testing.T) {
	m := newTestModel(fakeDiff(t, ""))
	m.refreshGit(context.Background())
	if sv := cardOf(t, m); sv.shown != checkout || sv.git.Branch != "b:"+checkout {
		t.Fatalf("before cd: shown %q branch %q, want the checkout's", sv.shown, sv.git.Branch)
	}

	send(m, "session-cwd", sessionCwd{ID: "s1", Cwd: elsewhere})
	m.refreshGit(context.Background())
	sv := cardOf(t, m)
	if sv.shown != elsewhere || sv.git.Branch != "b:"+elsewhere || sv.git.Added != 3 || sv.git.Deleted != 2 {
		t.Fatalf("after cd: got shown %q git %+v, want %q's", sv.shown, sv.git, elsewhere)
	}
	if sv.path != checkout {
		t.Errorf("group path moved to %q; a cd must not regroup the card", sv.path)
	}
}

func TestCardOutsideGitShowsNoBranch(t *testing.T) {
	m := newTestModel(fakeDiff(t, elsewhere))
	send(m, "session-cwd", sessionCwd{ID: "s1", Cwd: elsewhere})
	m.refreshGit(context.Background())
	if sv := cardOf(t, m); sv.git.Branch != "" {
		t.Errorf("got branch %q for a directory outside git, want none", sv.git.Branch)
	}
}

func TestCardCarriesUnreadableHost(t *testing.T) {
	m := newTestModel(fakeDiff(t, ""))
	send(m, "session-cwd", sessionCwd{ID: "s1", Cwd: checkout, Host: "tmux"})
	if sv := cardOf(t, m); sv.host != "tmux" {
		t.Errorf("got host %q, want tmux", sv.host)
	}
}

func TestDisplayPathCollapsesHome(t *testing.T) {
	cases := map[string]string{
		"/home/me/try/skipo":   "~/try/skipo",
		"/Users/me":            "~",
		`C:\Users\me\src`:      `~\src`,
		"/opt/home/me":         "/opt/home/me",
		"/var/tmp/home/me/try": "/var/tmp/home/me/try",
	}
	for in, want := range cases {
		if got := displayPath(in); got != want {
			t.Errorf("displayPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCloseSessionActivatesNeighbor(t *testing.T) {
	ids := func(m *model) (out []string) {
		for _, s := range m.projects[0].Sessions {
			out = append(out, s.ID)
		}
		return out
	}
	m := newModel(nil, func() {})
	m.projects = []lichclient.Project{{ID: "p", Sessions: []lichclient.Session{{ID: "a"}, {ID: "b"}, {ID: "c"}}}}

	m.active = "b"
	if p, active, ok := m.closeSession("b"); !ok || p != "p" || active != "c" {
		t.Fatalf("closing the middle: got %q %q %v, want p c true", p, active, ok)
	}
	if _, active, _ := m.closeSession("c"); active != "a" {
		t.Fatalf("closing the last: got active %q, want the one before, a", active)
	}
	m.active = "z"
	if _, active, _ := m.closeSession("a"); active != "z" {
		t.Errorf("closing a background card moved the active one to %q", active)
	}
	if len(ids(m)) != 0 {
		t.Errorf("left %v open", ids(m))
	}
	if _, _, ok := m.closeSession("a"); ok {
		t.Error("closing an unknown id reported ok")
	}
}

func TestFollowMarksTheWindowLostWhenTheStreamEnds(t *testing.T) {
	m := newModel(nil, func() {})
	events := make(chan lichclient.Event)
	close(events)
	m.follow(context.Background(), events)
	if !m.connectionLost() {
		t.Error("the stream ended under a live window and the model is not lost")
	}
}

func TestFollowStaysQuietWhenTheWindowCloses(t *testing.T) {
	m := newModel(nil, func() {})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	events := make(chan lichclient.Event)
	close(events)
	m.follow(ctx, events)
	if m.connectionLost() {
		t.Error("the window closing itself marked the model lost")
	}
}
