package terminal

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/omartelo/lich/internal/events"
)

// An agent CLI the session's agent runs as a tool (`claude -p`, `cursor-agent
// -p`) inherits the session's LICH_* environment and runs the same plugin, so
// its reports name the host card. What tells them apart is the conversation id
// each report carries (docs/hooks/session-state.md, Nested agent CLIs).

func TestHookDropsAReportFromAnotherConversation(t *testing.T) {
	svc, states := nestedRig(t, "host")

	postReport(t, svc, `{"session_id":"s1","state":"busy","provider_session_id":"nested"}`)
	postReport(t, svc, `{"session_id":"s1","state":"done","provider_session_id":"host"}`)
	postReport(t, svc, `{"session_id":"s1","state":"busy"}`)

	want := []string{statusDone, statusBusy}
	if got := states.seen(); !slices.Equal(got, want) {
		t.Fatalf("the relay heard %v, want %v", got, want)
	}
}

// The measured failure: a nested CLI exiting reported SessionEnd as the host's
// `idle`, and the relay closed every errand delivered to the host.
func TestNestedSessionEndDoesNotReachTheRelay(t *testing.T) {
	svc, states := nestedRig(t, "host")

	postReport(t, svc, `{"session_id":"s1","state":"busy","provider_session_id":"host"}`)
	postStart(t, svc, "nested")
	postReport(t, svc, `{"session_id":"s1","state":"busy","provider_session_id":"nested"}`)
	postReport(t, svc, `{"session_id":"s1","state":"done","provider_session_id":"nested"}`)
	postReport(t, svc, `{"session_id":"s1","state":"idle","provider_session_id":"nested"}`)

	if got := states.seen(); !slices.Equal(got, []string{statusBusy}) {
		t.Fatalf("the relay heard %v, want only the host's busy", got)
	}
	if bound := svc.store.(*boundStore).bound(); bound != "host" {
		t.Fatalf("a session-start inside the host's turn rebound it to %q", bound)
	}
}

// `/clear` (and `/resume`, `/new`) starts another conversation at the prompt,
// with no turn open: that start is the session's own and moves the binding.
func TestStartBetweenTurnsRebindsTheSession(t *testing.T) {
	svc, states := nestedRig(t, "host")

	postReport(t, svc, `{"session_id":"s1","state":"busy","provider_session_id":"host"}`)
	postReport(t, svc, `{"session_id":"s1","state":"done","provider_session_id":"host"}`)
	postReport(t, svc, `{"session_id":"s1","state":"idle","provider_session_id":"host"}`)
	postStart(t, svc, "cleared")
	postReport(t, svc, `{"session_id":"s1","state":"busy","provider_session_id":"cleared"}`)
	postReport(t, svc, `{"session_id":"s1","state":"busy","provider_session_id":"host"}`)

	if bound := svc.store.(*boundStore).bound(); bound != "cleared" {
		t.Fatalf("bound = %q after a start between turns, want %q", bound, "cleared")
	}
	want := []string{statusBusy, statusDone, statusIdle, statusBusy}
	if got := states.seen(); !slices.Equal(got, want) {
		t.Fatalf("the relay heard %v, want %v", got, want)
	}
}

// The first start binds whenever it lands: a provider that reports its id late
// (Crush, on its first tool call) has nothing bound to be confused with.
func TestFirstStartBindsInsideATurn(t *testing.T) {
	svc, _ := nestedRig(t, "")

	postReport(t, svc, `{"session_id":"s1","state":"busy"}`)
	postStart(t, svc, "first")

	if bound := svc.store.(*boundStore).bound(); bound != "first" {
		t.Fatalf("bound = %q, want %q", bound, "first")
	}
}

func nestedRig(t *testing.T, bound string) (*Service, *stateLog) {
	t.Helper()
	store := &boundStore{id: bound}
	svc := New(store, nil, events.New())
	if svc.wsErr != nil {
		t.Fatalf("transport: %v", svc.wsErr)
	}
	states := &stateLog{}
	svc.SetSessionState(func(_, state string) { states.add(state) })
	return svc, states
}

// boundStore is hookStore holding the one provider conversation id session-start
// writes and every report is checked against.
type boundStore struct {
	hookStore
	mu sync.Mutex
	id string
}

func (s *boundStore) SetProviderSession(_, providerSessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.id = providerSessionID
	return nil
}

func (s *boundStore) ProviderSession(string) (string, error) { return s.bound(), nil }

func (s *boundStore) bound() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.id
}

type stateLog struct {
	mu     sync.Mutex
	states []string
}

func (l *stateLog) add(state string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.states = append(l.states, state)
}

func (l *stateLog) seen() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.Clone(l.states)
}

func postReport(t *testing.T, svc *Service, body string) {
	t.Helper()
	postAccepted(t, svc, "/hook", body)
}

func postStart(t *testing.T, svc *Service, providerSessionID string) {
	t.Helper()
	postAccepted(t, svc, "/session-start",
		fmt.Sprintf(`{"session_id":"s1","provider_session_id":%q}`, providerSessionID))
}

func postAccepted(t *testing.T, svc *Service, path, body string) {
	t.Helper()
	url := fmt.Sprintf("http://127.0.0.1:%d%s?token=%s", svc.ws.port, path, svc.ws.token)
	resp, err := http.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("post %s %s: %v", path, body, err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("%s %s: status = %d, want 204", path, body, resp.StatusCode)
	}
}
