package terminal

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/omartelo/lich/internal/events"
)

func TestModAnswersOnlyFromAPollingModOfARelease(t *testing.T) {
	tests := []struct {
		name    string
		release string
		polled  bool
		want    bool
	}{
		{"the first release that answers", "0.17.0", true, true},
		{"a later one", "0.17.3", true, true},
		{"a release before it", "0.16.0", true, false},
		{"no release named", "", true, false},
		{"no poll", "0.17.0", false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := newModService(t, events.New(), 10*time.Millisecond)
			runModSession(svc, "s1")
			if tc.polled {
				pollMod(t, svc.ws, "s1")
			}
			svc.ws.plugins.note("s1", tc.release)
			if got := svc.ModAnswers("s1"); got != tc.want {
				t.Fatalf("ModAnswers = %v, want %v", got, tc.want)
			}
		})
	}
	if (&Service{}).ModAnswers("s1") {
		t.Error("a service with no transport answers")
	}
}

func TestModAnswerEndpointHandsTheTextToTheRelay(t *testing.T) {
	svc := New(stubBins{}, nil, events.New())
	if svc.ws == nil {
		t.Fatalf("transport: %v", svc.wsErr)
	}
	type answer struct{ id, text string }
	got := make(chan answer, 1)
	svc.SetWorkerAnswer(func(id, text string) { got <- answer{id, text} })
	url := fmt.Sprintf("http://127.0.0.1:%d/mod/answer?token=%s", svc.ws.port, svc.ws.token)

	resp, err := http.Post(url, "application/json", strings.NewReader(`{"session_id":"s1","text":"Done."}`))
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}
	select {
	case a := <-got:
		if a != (answer{"s1", "Done."}) {
			t.Errorf("relay got %+v", a)
		}
	case <-time.After(time.Second):
		t.Fatal("the relay never got the answer")
	}
}

// The relay hears about a close while the process still runs, so it can end a
// worker's errand before the SessionEnd the dying CLI reports.
func TestCloseTellsTheWatcherBeforeTheProcessDies(t *testing.T) {
	svc := newModService(t, events.New(), 10*time.Millisecond)
	p := runModSession(svc, "s1")
	var order []string
	svc.SetSessionClosed(func(id string) { order = append(order, "closed "+id) })
	p.onClose = func() { order = append(order, "killed") }

	if err := svc.Close("s1"); err != nil {
		t.Fatalf("close: %v", err)
	}

	if strings.Join(order, ", ") != "closed s1, killed" {
		t.Errorf("order = %q, want the watcher told before the kill", order)
	}
}
