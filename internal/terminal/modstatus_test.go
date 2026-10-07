package terminal

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/omartelo/lich/internal/events"
	"github.com/omartelo/lich/internal/relay"
)

func modStatusURL(tr *transport, session string) string {
	return fmt.Sprintf("http://127.0.0.1:%d/mod/status?token=%s&session_id=%s", tr.port, tr.token, session)
}

// getModStatus reads /mod/status and returns the status code and the body.
func getModStatus(t *testing.T, url string) (int, []byte) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, body
}

func TestModStatusRefusesBadRequests(t *testing.T) {
	tr := newNilTransport(t)
	tests := []struct {
		name   string
		method string
		url    string
		want   int
	}{
		{"not a GET", http.MethodPost, modStatusURL(tr, "s1"), http.StatusMethodNotAllowed},
		{"bad token", http.MethodGet,
			fmt.Sprintf("http://127.0.0.1:%d/mod/status?token=wrong&session_id=s1", tr.port), http.StatusUnauthorized},
		{"missing session_id", http.MethodGet,
			fmt.Sprintf("http://127.0.0.1:%d/mod/status?token=%s", tr.port, tr.token), http.StatusBadRequest},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(tc.method, tc.url, nil)
			if err != nil {
				t.Fatalf("new request: %v", err)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("do: %v", err)
			}
			_ = resp.Body.Close()
			if resp.StatusCode != tc.want {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tc.want)
			}
		})
	}
}

// The contract promises three lists, never null, even before the relay is
// wired.
func TestModStatusAnswersEmptyListsWithNothingWired(t *testing.T) {
	tr := newNilTransport(t)

	code, body := getModStatus(t, modStatusURL(tr, "s1"))

	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if got, want := string(body), `{"owed":[],"open":[],"ready":[]}`+"\n"; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}

func TestModStatusAnswersWithTheCallingSessionsErrands(t *testing.T) {
	svc := newModService(t, events.New(), modPollWait)
	var asked string
	svc.SetErrandStatus(func(id string) relay.Status {
		asked = id
		return relay.Status{
			Owed:  []relay.OwedErrand{{Ticket: "t1", From: "lead", Asked: "write the docs"}},
			Open:  []relay.OpenErrand{},
			Ready: []relay.ReadyErrand{{Ticket: "t2", Target: "docs", Status: relay.StatusAnswered}},
		}
	})

	code, body := getModStatus(t, modStatusURL(svc.ws, "s1"))

	if code != http.StatusOK || asked != "s1" {
		t.Fatalf("status = %d for session %q, want 200 for s1", code, asked)
	}
	want := `{"owed":[{"ticket":"t1","from":"lead","asked":"write the docs"}],"open":[],` +
		`"ready":[{"ticket":"t2","target":"docs","status":"answered"}]}` + "\n"
	if string(body) != want {
		t.Fatalf("body = %s, want %s", body, want)
	}
}

// TestModStatusMatchesFixture pins the lich-to-mod direction: the mod reads
// docs/hooks/fixtures/mod-status.json, so the shape lich sends must encode to
// exactly what that file says.
func TestModStatusMatchesFixture(t *testing.T) {
	status := relay.Status{
		Owed: []relay.OwedErrand{
			{Ticket: "b19cb405", From: "Session 26", Asked: "Sou a sessão Claude do lich-plugin"},
			{Ticket: "e2d4f6a8", From: "", Asked: "If a < b && c > d, see </result>."},
		},
		Open: []relay.OpenErrand{
			{Ticket: "4f0c1a2e", Target: "docs", State: "busy"},
			{Ticket: "5a6b7c8d", Target: "lint", State: "waiting"},
			{Ticket: "6e7f8091", Target: "build", State: ""},
			{Ticket: "9d3e77b0", Target: "tests", State: "queued"},
		},
		Ready: []relay.ReadyErrand{
			{Ticket: "c7a91e04", Target: "review", Status: relay.StatusAnswered},
			{Ticket: "d1e2f3a4", Target: "a<b&c", Status: relay.StatusUnanswered},
		},
	}
	encoded, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	golden, err := os.ReadFile(filepath.Join(hookFixtureDir, "mod-status.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var got, want map[string]any
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatalf("decode encoded: %v", err)
	}
	if err := json.Unmarshal(golden, &want); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("lich sends %v, the fixture says %v", got, want)
	}
}
