package relay

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

// plantPrivate is plant for a ticket sent with SendPrivate.
func plantPrivate(svc *Service, id, fromID, targetID, target string) {
	plant(svc, id, fromID, targetID, target)
	svc.mu.Lock()
	svc.tickets[id].private = true
	svc.mu.Unlock()
}

func TestSendPrivateMarksItsResult(t *testing.T) {
	svc := newRelay(workspace(), newFakeTerminal("s1", "s2"), nil)

	var (
		got Result
		err error
		wg  sync.WaitGroup
	)
	wg.Go(func() {
		got, err = svc.SendPrivate(context.Background(), "s1", "docs", "", "run the tests", 30)
	})
	if replyErr := svc.Reply("", waitForTicket(svc), "green"); replyErr != nil {
		t.Fatalf("Reply: %v", replyErr)
	}
	wg.Wait()

	if err != nil {
		t.Fatalf("SendPrivate: %v", err)
	}
	if got.Status != StatusAnswered || got.Answer != "green" || !got.Private {
		t.Errorf("SendPrivate = %+v, want a private answered result", got)
	}
}

// A private result is its ticket's alone: a no-ticket collect from the same
// session, which is how a workflow agent's sibling or the session's own agent
// drains its inbox, must not take it, and the ticket still finds it after.
func TestANoTicketCollectNeverTakesAPrivateResult(t *testing.T) {
	svc := newRelay(workspace(), newFakeTerminal("s1", "s2"), nil)
	plantPrivate(svc, "t1", "s1", "s2", "docs")
	if err := svc.Reply("", "t1", "done"); err != nil {
		t.Fatalf("Reply: %v", err)
	}

	collected, err := svc.CollectNow("s1")
	if err != nil {
		t.Fatalf("CollectNow: %v", err)
	}
	if len(collected.Results) != 0 || len(collected.Open) != 0 {
		t.Fatalf("CollectNow = %+v, want the private result left alone", collected)
	}

	got, err := svc.Wait(context.Background(), "t1", 1)
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if got.Status != StatusAnswered || got.Answer != "done" || !got.Private {
		t.Errorf("Wait = %+v, want the private answer", got)
	}
}

// The session's prompt does not speak for whoever sent privately, so nothing is
// typed there and the card counts nothing it could not collect.
func TestAPrivateResultIsNeverAnnounced(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	events := &fakeEvents{}
	svc := newRelay(workspace(), term, events)
	plantPrivate(svc, "t1", "s1", "s2", "docs")
	if err := svc.Reply("", "t1", "done"); err != nil {
		t.Fatalf("Reply: %v", err)
	}
	svc.Observe("s1", stateDone)
	time.Sleep(20 * svc.nudgeDelay)

	if typed := term.written("s1"); strings.Contains(typed, "[lich]") {
		t.Errorf("the session was nudged about a private result: %q", typed)
	}
	events.mu.Lock()
	defer events.mu.Unlock()
	for _, e := range events.inbox {
		if e.ID == "s1" && e.Count != 0 {
			t.Errorf("the card counted %d result(s), want none it could collect", e.Count)
		}
	}
}

func TestACollectDoesNotHoldTheLineForAPrivateErrand(t *testing.T) {
	svc := newRelay(workspace(), newFakeTerminal("s1", "s2"), nil)
	plantPrivate(svc, "t1", "s1", "s2", "docs")

	start := time.Now()
	collected, err := svc.Collect(context.Background(), "s1", 5)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("Collect held the line %v for an errand it can never drain", elapsed)
	}
	if len(collected.Open) != 0 {
		t.Errorf("Open = %v, want the private errand left out", collected.Open)
	}
}

func TestAnOrdinaryResultIsStillNudgedBesideAPrivateOne(t *testing.T) {
	term := newFakeTerminal("s1", "s2", "s3")
	svc := newRelay(workspace(), term, nil)
	plantPrivate(svc, "t1", "s1", "s2", "docs")
	plant(svc, "t2", "s1", "s3", "api")
	_ = svc.Reply("", "t1", "private")
	_ = svc.Reply("", "t2", "ordinary")

	if !awaitWritten(term, "s1", "[lich]") {
		t.Fatalf("the ordinary result was never announced: %q", term.written("s1"))
	}
	if typed := term.written("s1"); strings.Contains(typed, `"docs"`) || !strings.Contains(typed, `"api"`) {
		t.Errorf("nudge = %q, want it to name api and not the private docs", typed)
	}
	collected, _ := svc.CollectNow("s1")
	if len(collected.Results) != 1 || collected.Results[0].Answer != "ordinary" {
		t.Errorf("CollectNow = %+v, want the ordinary result only", collected)
	}
}

// Held for its ticket like any result, for as long, and dropped without a word:
// nobody was ever told it existed.
func TestAPrivateResultAgesOutSilently(t *testing.T) {
	events := &fakeEvents{}
	svc := newRelay(workspace(), newFakeTerminal("s1", "s2"), events)
	plantPrivate(svc, "t1", "s1", "s2", "docs")
	_ = svc.Reply("", "t1", "done")

	later := time.Now().Add(2 * ticketTTL)
	svc.now = func() time.Time { return later }
	if got, err := svc.Wait(context.Background(), "t1", 1); err != nil || got.Status != StatusExpired {
		t.Fatalf("Wait = %+v, %v, want the private result expired past its ticket's lifetime", got, err)
	}
	events.mu.Lock()
	defer events.mu.Unlock()
	for _, e := range events.inbox {
		if e.Count != 0 {
			t.Errorf("inbox events = %+v, want nothing counted for a result nobody was told about", events.inbox)
		}
	}
}

// Nobody else may hear of it, but the caller that comes back on the ticket
// must: told "unknown ticket", it cannot tell a result it missed from a ticket
// it mistyped.
func TestAWaitOnAnExpiredPrivateResultSaysItExpired(t *testing.T) {
	svc := newRelay(workspace(), newFakeTerminal("s1", "s2"), nil)
	plantPrivate(svc, "t1", "s1", "s2", "docs")
	_ = svc.Reply("", "t1", "done")

	later := time.Now().Add(2 * ticketTTL)
	svc.now = func() time.Time { return later }
	got, err := svc.Wait(context.Background(), "t1", 1)
	if err != nil || got.Status != StatusExpired || !got.Private {
		t.Fatalf("Wait = %+v, %v, want the private result expired", got, err)
	}
	if _, err := svc.Wait(context.Background(), "never", 1); err == nil || !strings.Contains(err.Error(), "unknown ticket") {
		t.Errorf("Wait on a ticket that never existed = %v, want unknown ticket", err)
	}
}
