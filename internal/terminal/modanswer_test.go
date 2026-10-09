package terminal

import (
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

// The relay tells a closed worker's caller it stopped unless the caller closed
// it, so the watcher hears who closed the session; the window's close names no
// one.
func TestCloseTellsTheWatcherWhoClosedTheSession(t *testing.T) {
	svc := newModService(t, events.New(), 10*time.Millisecond)
	runModSession(svc, "s1")
	runModSession(svc, "s2")
	var closers []string
	svc.SetSessionClosed(func(id, closerID string) { closers = append(closers, id+" by "+closerID) })

	if err := svc.CloseBy("s1", "s9"); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := svc.Close("s2"); err != nil {
		t.Fatalf("close: %v", err)
	}

	if got := strings.Join(closers, ", "); got != "s1 by s9, s2 by " {
		t.Errorf("closers = %q, want s1 by s9 and s2 by nobody", got)
	}
}

// The relay hears about a close while the process still runs, so it can end a
// worker's errand before the SessionEnd the dying CLI reports.
func TestCloseTellsTheWatcherBeforeTheProcessDies(t *testing.T) {
	svc := newModService(t, events.New(), 10*time.Millisecond)
	p := runModSession(svc, "s1")
	var order []string
	svc.SetSessionClosed(func(id, _ string) { order = append(order, "closed "+id) })
	p.onClose = func() { order = append(order, "killed") }

	if err := svc.Close("s1"); err != nil {
		t.Fatalf("close: %v", err)
	}

	if strings.Join(order, ", ") != "closed s1, killed" {
		t.Errorf("order = %q, want the watcher told before the kill", order)
	}
}
