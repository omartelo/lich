package relay

import (
	"sync"
	"testing"
	"time"
)

// parkWriter records every scheduled-prompt write, parks included.
type parkWriter struct {
	mu     sync.Mutex
	writes []ScheduleEvent
}

func (w *parkWriter) set(sessionID string, at int64, prompt string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.writes = append(w.writes, ScheduleEvent{ID: sessionID, At: at, Prompt: prompt})
	return nil
}

func (w *parkWriter) all() []ScheduleEvent {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]ScheduleEvent(nil), w.writes...)
}

func TestParkResumeParksTheContinuationAfterTheReset(t *testing.T) {
	writer := &parkWriter{}
	events := &fakeEvents{}
	svc := newRelay(scheduledWith(0, "", writer.set), newFakeTerminal("s1"), events)
	svc.now = at(1000)

	svc.ParkResume("s1", 1000+3600)

	want := ScheduleEvent{ID: "s1", At: 1000 + 3600 + int64(resumeGrace/time.Second), Prompt: resumePrompt}
	if got := writer.all(); len(got) != 1 || got[0] != want {
		t.Fatalf("writes = %+v, want [%+v]", got, want)
	}
	if got := events.schedules(); len(got) != 1 || got[0] != want {
		t.Fatalf("schedule events = %+v, want [%+v]", got, want)
	}
}

func TestParkResumeParksNothingItShouldNotWaitFor(t *testing.T) {
	tests := []struct {
		name     string
		resetsAt int64
		prompt   string
	}{
		{"a reset already past", 999, ""},
		{"a reset beyond the horizon", 1000 + int64(resumeHorizon/time.Second) + 1, ""},
		{"a prompt the person parked", 1000 + 3600, "ship the release notes"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			writer := &parkWriter{}
			svc := newRelay(scheduledWith(5000, tc.prompt, writer.set), newFakeTerminal("s1"), &fakeEvents{})
			svc.now = at(1000)

			svc.ParkResume("s1", tc.resetsAt)

			if got := writer.all(); len(got) != 0 {
				t.Fatalf("writes = %+v, want none", got)
			}
		})
	}
}

// A turn starting before the reset makes the continuation moot — the person
// went on by hand, or Claude Code resumed on its own — and it is dropped rather
// than typed behind that turn. Learned from the row too, so a continuation
// parked before a restart is dropped all the same.
func TestATurnStartingDropsTheParkedContinuation(t *testing.T) {
	writer := &parkWriter{}
	events := &fakeEvents{}
	svc := newRelay(scheduledWith(5000, resumePrompt, writer.set), newFakeTerminal("s1"), events)
	svc.now = at(1000)
	svc.deliverDue()

	svc.Observe("s1", stateBusy)

	waitUntil(t, func() bool { return len(writer.all()) == 1 })
	if got := writer.all()[0]; got != (ScheduleEvent{ID: "s1"}) {
		t.Fatalf("write = %+v, want the row cleared", got)
	}
	if got := events.schedules(); len(got) != 1 || got[0] != (ScheduleEvent{ID: "s1"}) {
		t.Fatalf("schedule events = %+v, want one clearing s1", got)
	}
}

func TestATurnStartingLeavesThePersonsPromptAlone(t *testing.T) {
	writer := &parkWriter{}
	svc := newRelay(scheduledWith(5000, "ship the release notes", writer.set), newFakeTerminal("s1"), &fakeEvents{})
	svc.now = at(1000)
	svc.markResume("s1")

	svc.dropResume("s1")

	if got := writer.all(); len(got) != 0 {
		t.Fatalf("writes = %+v, want the person's prompt kept", got)
	}
}

func scheduledWith(at int64, prompt string, set func(string, int64, string) error) fakeSessions {
	sessions := scheduled(at, prompt, &scheduleWriter{})
	sessions.schedule = set
	return sessions
}

func waitUntil(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition never held")
		}
		time.Sleep(time.Millisecond)
	}
}
