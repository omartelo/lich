//go:build !windows

package terminal

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/omartelo/lich/internal/events"
)

// Unix-only with the stubBins it reads, which live in the PTY suite.

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

func TestModAnswerEndpointHandsAnUnansweredTurnToTheRelay(t *testing.T) {
	svc := New(stubBins{}, nil, events.New())
	if svc.ws == nil {
		t.Fatalf("transport: %v", svc.wsErr)
	}
	type unanswered struct{ id, reason string }
	got := make(chan unanswered, 1)
	svc.SetWorkerAnswer(func(id, text string) { t.Errorf("answered %s with %q", id, text) })
	svc.SetWorkerUnanswered(func(id, reason string) { got <- unanswered{id, reason} })
	url := fmt.Sprintf("http://127.0.0.1:%d/mod/answer?token=%s", svc.ws.port, svc.ws.token)

	resp, err := http.Post(url, "application/json", strings.NewReader(`{"session_id":"s1","unanswered":"refusal"}`))
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}
	select {
	case u := <-got:
		if u != (unanswered{"s1", "refusal"}) {
			t.Errorf("relay got %+v", u)
		}
	case <-time.After(time.Second):
		t.Fatal("the relay never heard the turn went unanswered")
	}
}
