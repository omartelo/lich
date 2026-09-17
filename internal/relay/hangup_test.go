package relay

import (
	"context"
	"errors"
	"testing"
	"time"
)

// A caller that hangs up mid-wait (Esc on a tool call, Ctrl-C on `lich wait`)
// must stop taking results: whatever it would have been handed nobody reads, and
// the nudge that would have announced it was skipped for its sake.

// awaitHolding blocks until n collectors are holding the line for fromID.
func awaitHolding(t *testing.T, svc *Service, fromID string, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		svc.mu.Lock()
		holding := len(svc.collectors[fromID])
		svc.mu.Unlock()
		if holding == n {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("never saw %d collectors holding the line for %s", n, fromID)
}

// awaitAttended blocks until a ticket has n callers blocked on it.
func awaitAttended(t *testing.T, svc *Service, id string, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		svc.mu.Lock()
		attended := -1
		if tk, ok := svc.tickets[id]; ok {
			attended = tk.attended
		}
		svc.mu.Unlock()
		if attended == n {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("ticket %s never had %d waiters", id, n)
}

// assertStashedAndNudged proves an answer went where an unattended one goes: a
// nudge at the sender's prompt, and the inbox.
func assertStashedAndNudged(t *testing.T, svc *Service, term *fakeTerminal, answer string) {
	t.Helper()
	if !awaitWritten(term, "s1", "[lich]") {
		t.Errorf("the sender was never nudged: %q", term.written("s1"))
	}
	collected, err := svc.CollectNow("s1")
	if err != nil {
		t.Fatalf("CollectNow: %v", err)
	}
	if len(collected.Results) != 1 || collected.Results[0].Answer != answer {
		t.Fatalf("inbox holds %+v, want %q", collected, answer)
	}
}

func TestACollectWhoseCallerHungUpLeavesTheNextResultToTheNudge(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	svc := newRelay(workspace(), term, nil)
	plant(svc, "t1", "s1", "s2", "docs")

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := svc.Collect(ctx, "s1", 30)
		done <- err
	}()
	awaitHolding(t, svc, "s1", 1)
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("Collect = %v, want the hang-up reported", err)
	}
	awaitHolding(t, svc, "s1", 0)

	if err := svc.Reply("", "t1", "all green"); err != nil {
		t.Fatalf("Reply: %v", err)
	}
	assertStashedAndNudged(t, svc, term, "all green")
}

// The wake and the hang-up can cross: stash wakes the collector and skips the
// nudge, and the collector then finds its caller gone. The result must not be
// drained, and something still has to say it is there.
func TestAResultTheHungUpCollectorWasWokenForIsStillNudged(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	svc := newRelay(workspace(), term, nil)
	svc.mu.Lock()
	svc.ready["t1"] = &inboxEntry{
		ticket: "t1", fromID: "s1", target: "docs", status: StatusAnswered, answer: "all green", ready: time.Now(),
	}
	svc.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := svc.Collect(ctx, "s1", 30); !errors.Is(err, context.Canceled) {
		t.Fatalf("Collect = %v, want the hang-up reported", err)
	}
	assertStashedAndNudged(t, svc, term, "all green")
}

func TestARenudgeLeavesAResultToTheCollectorStillHolding(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	svc := newRelay(workspace(), term, nil)
	svc.mu.Lock()
	svc.ready["t1"] = &inboxEntry{ticket: "t1", fromID: "s1", target: "docs", ready: time.Now()}
	svc.collectors["s1"] = []chan struct{}{make(chan struct{}, 1)}
	svc.mu.Unlock()

	svc.renudge("s1")
	svc.mu.Lock()
	armed := svc.nudgeTimer["s1"] != nil
	svc.mu.Unlock()
	if armed {
		t.Error("a nudge was armed for a result a collector is about to carry out")
	}
}

func TestCollectNowReturnsAtOnceWithWhoStillOwes(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	svc := newRelay(workspace(), term, nil)
	plant(svc, "t1", "s1", "s2", "docs")

	start := time.Now()
	collected, err := svc.CollectNow("s1")
	if err != nil {
		t.Fatalf("CollectNow: %v", err)
	}
	if time.Since(start) > time.Second {
		t.Error("CollectNow held the line")
	}
	if len(collected.Results) != 0 || len(collected.Open) != 1 || collected.Open[0] != "docs" {
		t.Fatalf("collected %+v, want nothing ready and docs still owing", collected)
	}

	// Having looked is not holding the line: the next result is announced the
	// usual way instead of waking a collector that already returned.
	if err := svc.Reply("", "t1", "all green"); err != nil {
		t.Fatalf("Reply: %v", err)
	}
	assertStashedAndNudged(t, svc, term, "all green")
}

func TestCollectNowRefusesACallerWithNoSession(t *testing.T) {
	svc := newRelay(workspace(), newFakeTerminal(), nil)

	if _, err := svc.CollectNow(""); err == nil {
		t.Fatal("collected for a caller that has no inbox")
	}
}

func TestAWaitWhoseCallerHungUpLeavesTheAnswerToTheInbox(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	svc := newRelay(workspace(), term, nil)
	plant(svc, "t1", "s1", "s2", "docs")

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan Result, 1)
	go func() {
		res, _ := svc.Wait(ctx, "t1", 30)
		done <- res
	}()
	awaitAttended(t, svc, "t1", 1)
	cancel()
	if res := <-done; res.Status != StatusPending {
		t.Fatalf("the hung-up wait was handed %+v", res)
	}
	awaitAttended(t, svc, "t1", 0)

	if err := svc.Reply("", "t1", "all green"); err != nil {
		t.Fatalf("Reply: %v", err)
	}
	assertStashedAndNudged(t, svc, term, "all green")
}

// A wait that runs out comes back empty-handed with who still owes, once. The
// frozen clock is the case the timer path has to end on its own: a deadline
// read off a clock that never moves would otherwise hold the line forever.
func TestACollectWhoseWaitRunsOutReturnsWhoStillOwes(t *testing.T) {
	for _, tt := range []struct {
		name  string
		clock func() time.Time
	}{
		{"real clock", time.Now},
		{"frozen clock", at(1000)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			term := newFakeTerminal("s1", "s2")
			svc := newRelay(workspace(), term, nil)
			plant(svc, "t1", "s1", "s2", "docs")
			svc.now = tt.clock

			done := make(chan Collected, 1)
			go func() {
				collected, _ := svc.Collect(context.Background(), "s1", 1)
				done <- collected
			}()
			select {
			case collected := <-done:
				if len(collected.Results) != 0 || len(collected.Open) != 1 || collected.Open[0] != "docs" {
					t.Fatalf("collected %+v, want nothing ready and docs still owing", collected)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("Collect held the line past the wait it was given")
			}
			awaitHolding(t, svc, "s1", 0)
		})
	}
}

// An abandoned ticket wait whose errand closed without an answer (here: typed
// and never read) had that news meant for its caller; with the caller gone it
// goes to the inbox.
func TestAnAbandonedWaitStashesTheOutcomeItWasHolding(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	svc := newRelay(workspace(), term, nil)
	plant(svc, "t1", "s1", "s2", "docs")
	svc.mu.Lock()
	tk := svc.tickets["t1"]
	tk.attended = 1
	delete(svc.tickets, "t1")
	close(tk.unread)
	svc.mu.Unlock()

	svc.abandon("t1", tk)

	if !awaitWritten(term, "s1", "[lich]") {
		t.Errorf("the sender was never nudged: %q", term.written("s1"))
	}
	collected, err := svc.CollectNow("s1")
	if err != nil {
		t.Fatalf("CollectNow: %v", err)
	}
	if len(collected.Results) != 1 || collected.Results[0].Status != StatusUnread {
		t.Fatalf("inbox holds %+v, want the unread outcome", collected)
	}
}

// Another caller still holding the ticket carries the outcome out itself, so
// one hanging up must not also stash it.
func TestAnAbandonedWaitLeavesTheOutcomeToAnotherWaiter(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	svc := newRelay(workspace(), term, nil)
	plant(svc, "t1", "s1", "s2", "docs")
	svc.mu.Lock()
	tk := svc.tickets["t1"]
	tk.attended = 2
	delete(svc.tickets, "t1")
	close(tk.unread)
	svc.mu.Unlock()

	svc.abandon("t1", tk)

	collected, err := svc.CollectNow("s1")
	if err != nil {
		t.Fatalf("CollectNow: %v", err)
	}
	if len(collected.Results) != 0 {
		t.Fatalf("inbox holds %+v, want the outcome left to the waiter still there", collected)
	}
}

// A result held back for a busy sender is announced when its turn is stopped,
// not only when one finishes: Esc is how a wait for results is abandoned, and
// the next turn end may be a long way off.
func TestAnInterruptedSenderIsNudged(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	svc := newRelay(workspace(), term, nil)
	svc.Observe("s1", stateBusy)
	plant(svc, "t1", "s1", "s2", "docs")

	if err := svc.Reply("", "t1", "all green"); err != nil {
		t.Fatalf("Reply: %v", err)
	}
	time.Sleep(20 * time.Millisecond)
	if typed := term.written("s1"); typed != "" {
		t.Fatalf("nudged a sender mid-turn: %q", typed)
	}

	svc.Observe("s1", stateInterrupted)
	if !awaitWritten(term, "s1", "[lich]") {
		t.Fatalf("the turn was stopped and no nudge came: %q", term.writesTo("s1"))
	}
}
