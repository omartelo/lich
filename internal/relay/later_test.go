package relay

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/omartelo/lich/internal/store"
)

// scheduleWriter records what deliverDue wrote back to the session rows.
type scheduleWriter struct {
	mu      sync.Mutex
	cleared []string
	err     error
}

func (w *scheduleWriter) set(sessionID string, at int64, prompt string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.err != nil {
		return w.err
	}
	if at == 0 && prompt == "" {
		w.cleared = append(w.cleared, sessionID)
	}
	return nil
}

func (w *scheduleWriter) clearedRows() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]string(nil), w.cleared...)
}

// scheduled is a workspace holding one session with a prompt parked on it.
func scheduled(at int64, prompt string, writer *scheduleWriter) fakeSessions {
	return fakeSessions{
		projects: []store.Project{{ID: "p1", Name: "lich", Sessions: []store.Session{
			{ID: "s1", Label: "docs", Kind: "claude", ScheduledAt: at, ScheduledPrompt: prompt},
		}}},
		schedule: writer.set,
	}
}

// at is a clock stopped at a unix second, so a test says when "now" is rather
// than waiting for it.
func at(unix int64) func() time.Time {
	return func() time.Time { return time.Unix(unix, 0) }
}

func TestDeliverDueTypesThePromptAndClearsTheRow(t *testing.T) {
	writer := &scheduleWriter{}
	term := newFakeTerminal("s1")
	events := &fakeEvents{}
	svc := newRelay(scheduled(1000, "run the release checklist", writer), term, events)
	svc.now = at(1000)

	svc.deliverDue()

	written := term.written("s1")
	if !strings.Contains(written, "run the release checklist") {
		t.Fatalf("typed at s1 = %q, want the scheduled prompt", written)
	}
	if !strings.HasSuffix(written, submit) {
		t.Fatalf("typed at s1 = %q, want it sent", written)
	}
	if got := writer.clearedRows(); len(got) != 1 || got[0] != "s1" {
		t.Fatalf("cleared = %v, want [s1]", got)
	}
	if got := events.schedules(); len(got) != 1 || got[0].ID != "s1" || got[0].At != 0 {
		t.Fatalf("schedule events = %+v, want one clearing s1", got)
	}
}

// The prompt is typed as the user wrote it: no sender, no ticket, none of the
// envelope a relayed task carries. A scheduled prompt has nobody to report to.
func TestDeliverDueTypesNoEnvelope(t *testing.T) {
	term := newFakeTerminal("s1")
	svc := newRelay(scheduled(1000, "ship it", &scheduleWriter{}), term, nil)
	svc.now = at(1000)

	svc.deliverDue()

	if got := term.written("s1"); strings.Contains(got, "ticket") || strings.Contains(got, ToolReply) {
		t.Fatalf("typed at s1 = %q, want the prompt alone", got)
	}
}

func TestDeliverDueLeavesAPromptThatIsNotDueYet(t *testing.T) {
	writer := &scheduleWriter{}
	term := newFakeTerminal("s1")
	svc := newRelay(scheduled(1001, "later", writer), term, nil)
	svc.now = at(1000)

	svc.deliverDue()

	if got := term.writesTo("s1"); len(got) != 0 {
		t.Fatalf("typed at s1 = %v, want nothing a second early", got)
	}
	if got := writer.clearedRows(); len(got) != 0 {
		t.Fatalf("cleared = %v, want the row left alone", got)
	}
}

// Overdue is delivered, not dropped: this is the prompt that came due while
// lich was closed.
func TestDeliverDueTypesAnOverduePrompt(t *testing.T) {
	term := newFakeTerminal("s1")
	svc := newRelay(scheduled(1000, "the overdue one", &scheduleWriter{}), term, nil)
	svc.now = at(1000 + 3*24*60*60)

	svc.deliverDue()

	if !strings.Contains(term.written("s1"), "the overdue one") {
		t.Fatalf("typed at s1 = %q, want the overdue prompt", term.written("s1"))
	}
}

// A session with no prompt to type at keeps its schedule: the row is the only
// copy of what the user wrote, and the next pass is when it lands.
func TestDeliverDueKeepsThePromptWhileTheSessionIsNotReady(t *testing.T) {
	cases := map[string]func(*fakeTerminal){
		"no PTY":         func(term *fakeTerminal) { term.live["s1"] = false },
		"setting up":     func(term *fakeTerminal) { term.settingUp["s1"] = true },
		"user is typing": func(term *fakeTerminal) { term.typing["s1"] = true },
	}
	for name, park := range cases {
		t.Run(name, func(t *testing.T) {
			writer := &scheduleWriter{}
			term := newFakeTerminal("s1")
			park(term)
			svc := newRelay(scheduled(1000, "wait for me", writer), term, nil)
			svc.now = at(1000)

			svc.deliverDue()

			if got := term.writesTo("s1"); len(got) != 0 {
				t.Fatalf("typed at s1 = %v, want nothing", got)
			}
			if got := writer.clearedRows(); len(got) != 0 {
				t.Fatalf("cleared = %v, want the schedule kept", got)
			}
		})
	}
}

// The clear is what stops a second pass typing the same prompt again, so a
// clear that fails has to stop the delivery with it.
func TestDeliverDueSkipsWhenTheRowCannotBeCleared(t *testing.T) {
	writer := &scheduleWriter{err: errors.New("database is locked")}
	term := newFakeTerminal("s1")
	svc := newRelay(scheduled(1000, "run it", writer), term, nil)
	svc.now = at(1000)

	svc.deliverDue()

	if got := term.writesTo("s1"); len(got) != 0 {
		t.Fatalf("typed at s1 = %v, want nothing typed against an unclearable row", got)
	}
}

func TestDeliverDueIgnoresSessionsWithNothingScheduled(t *testing.T) {
	term := newFakeTerminal("s1", "s2", "s3", "s4")
	svc := newRelay(workspace(), term, nil)
	svc.now = at(1000)

	svc.deliverDue()

	for _, id := range []string{"s1", "s2", "s3", "s4"} {
		if got := term.writesTo(id); len(got) != 0 {
			t.Fatalf("typed at %s = %v, want nothing", id, got)
		}
	}
}

func TestDeliverDueSurvivesAnUnreadableWorkspace(t *testing.T) {
	svc := newRelay(fakeSessions{err: errors.New("no database")}, newFakeTerminal("s1"), nil)
	svc.deliverDue()
}
