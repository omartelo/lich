package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/omartelo/lich/native/lichclient"
)

func TestStandingOf(t *testing.T) {
	cases := []struct {
		in   *lichclient.BaseStatus
		want baseStanding
	}{
		{nil, baseStanding{}},
		{&lichclient.BaseStatus{Behind: 0}, baseStanding{}},
		{&lichclient.BaseStatus{Behind: 3}, baseStanding{count: 3}},
		{&lichclient.BaseStatus{Behind: 3, Conflicts: []string{"a", "b"}}, baseStanding{conflict: true, count: 2}},
	}
	for _, c := range cases {
		if got := standingOf(c.in); got != c.want {
			t.Errorf("standingOf(%+v) = %+v, want %+v", c.in, got, c.want)
		}
	}
}

// fakeForge serves a checkout whose branch and head the test moves, with the
// PR gh would answer at each head, so an answer never depends on when the
// lookup lands.
type fakeForge struct {
	mu           sync.Mutex
	branch, head string
	prs          map[string]*lichclient.PullRequest // head -> open PR
	hold         chan struct{}                      // while set, gh does not answer
}

func (f *fakeForge) set(branch, head string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.branch, f.head = branch, head
}

func (f *fakeForge) client(t *testing.T) *lichclient.Client {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		hold := f.hold
		f.mu.Unlock()
		if hold != nil && r.URL.Path == "/rpc/project.PullRequest" {
			<-hold
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		switch r.URL.Path {
		case "/rpc/project.Diff":
			_ = json.NewEncoder(w).Encode(lichclient.DiffStats{Branch: f.branch, Head: f.head})
		case "/rpc/project.BaseStatus":
			_ = json.NewEncoder(w).Encode(lichclient.BaseStatus{Behind: 2})
		case "/rpc/project.PullRequest":
			_ = json.NewEncoder(w).Encode(f.prs[f.head])
		default:
			t.Errorf("unexpected call %s", r.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	return lichclient.New(lichclient.Runtime{Port: port, Token: "tok"})
}

// prOnCard refreshes git and waits for the card's PR badge to read want.
func prOnCard(t *testing.T, m *model, want int) {
	t.Helper()
	m.refreshGit(context.Background())
	deadline := time.Now().Add(5 * time.Second)
	for {
		got := 0
		if pr := cardOf(t, m).pr; pr != nil {
			got = pr.Number
		}
		if got == want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("PR on card %d, want %d", got, want)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestCardFollowsThePullRequestOfItsBranch(t *testing.T) {
	forge := &fakeForge{prs: map[string]*lichclient.PullRequest{
		"c2": {Number: 7},
		"c4": {Number: 9},
	}}
	forge.set("feat", "c1")
	m := newTestModel(forge.client(t))
	prOnCard(t, m, 0)
	if b := cardOf(t, m).base; b != (baseStanding{count: 2}) {
		t.Fatalf("base %+v, want 2 behind", b)
	}

	forge.set("feat", "c2") // the session opened its PR
	prOnCard(t, m, 7)

	forge.mu.Lock()
	forge.hold = make(chan struct{})
	forge.mu.Unlock()
	forge.set("feat", "c3") // a new commit: the badge stays while gh is asked
	m.refreshGit(context.Background())
	if pr := cardOf(t, m).pr; pr == nil || pr.Number != 7 {
		t.Fatalf("new commit on the branch: PR %+v before gh answers, want #7 kept", pr)
	}
	forge.mu.Lock()
	close(forge.hold)
	forge.hold = nil
	forge.mu.Unlock()
	prOnCard(t, m, 0) // c3 has no open PR: merged

	forge.prs["c3"] = &lichclient.PullRequest{Number: 8}
	m.mu.Lock()
	m.stalePullRequests() // the window came back: opened in the browser
	m.mu.Unlock()
	prOnCard(t, m, 8)

	forge.mu.Lock()
	forge.hold = make(chan struct{})
	forge.mu.Unlock()
	forge.set("other", "c4")
	m.refreshGit(context.Background())
	if pr := cardOf(t, m).pr; pr != nil {
		t.Fatalf("new branch: PR #%d before gh answers, want the old branch's dropped", pr.Number)
	}
	forge.mu.Lock()
	close(forge.hold)
	forge.hold = nil
	forge.mu.Unlock()
	prOnCard(t, m, 9)
}
