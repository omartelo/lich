package relay

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

// A worker that hands its work to the background ends its turn before the work
// is done, and the errand is closed as unanswered at that turn's end. Its answer
// comes later, on the same ticket, and has to reach the sender rather than be
// refused as an unknown ticket.
func TestAnAnswerAfterTheTurnEndedWithoutOneStillReachesTheSender(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	svc := newRelay(workspace(), term, nil)

	sent, err := svc.Send(context.Background(), SendOptions{From: "s1", Target: "docs", Prompt: "run the tests", WaitSeconds: 1})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	svc.Observe("s2", "busy")
	svc.Observe("s2", "done")
	if stalled, err := svc.Wait(context.Background(), sent.Ticket, 1); err != nil || stalled.Status != StatusUnanswered {
		t.Fatalf("Wait = %+v, %v, want the turn's end reported unanswered first", stalled, err)
	}

	if err := svc.Reply("s2", sent.Ticket, "green, after the background run"); err != nil {
		t.Fatalf("a late Reply = %v, want the answer accepted", err)
	}
	if !awaitWritten(term, "s1", "[lich]") {
		t.Errorf("the sender was never told the late answer arrived: %q", term.written("s1"))
	}
	got, err := svc.Wait(context.Background(), sent.Ticket, 1)
	if err != nil {
		t.Fatalf("Wait for the late answer: %v", err)
	}
	if got.Status != StatusAnswered || got.Answer != "green, after the background run" {
		t.Errorf("Wait = %+v, want the late answer", got)
	}
}

// A late answer is still one answer: a second one on the same ticket is refused.
func TestALateAnswerIsTakenOnce(t *testing.T) {
	svc := newRelay(workspace(), newFakeTerminal("s1", "s2"), nil)
	sent, _ := svc.Send(context.Background(), SendOptions{From: "s1", Target: "docs", Prompt: "run the tests", WaitSeconds: 1})
	svc.Observe("s2", "busy")
	svc.Observe("s2", "done")

	if err := svc.Reply("s2", sent.Ticket, "first"); err != nil {
		t.Fatalf("late Reply: %v", err)
	}
	if err := svc.Reply("s2", sent.Ticket, "second"); err == nil {
		t.Error("a second late answer was accepted on the same ticket")
	}
}

// A ticket somebody is still waiting on is not expired from under them. The
// waiter used to be told the errand was still in progress on a ticket the sweep
// had already dropped, so the next wait on it answered "unknown ticket".
func TestATicketBeingWaitedOnOutlivesItsHour(t *testing.T) {
	svc := newRelay(workspace(), newFakeTerminal("s1", "s2"), nil)
	plant(svc, "t1", "s1", "s2", "docs")
	start := time.Now()
	clock := start
	var clockMu sync.Mutex
	svc.now = func() time.Time {
		clockMu.Lock()
		defer clockMu.Unlock()
		return clock
	}

	waited := make(chan Result, 1)
	go func() {
		got, _ := svc.Wait(context.Background(), "t1", 1)
		waited <- got
	}()
	awaitAttended(t, svc, "t1", 1)
	clockMu.Lock()
	clock = start.Add(2 * ticketTTL)
	clockMu.Unlock()
	if _, err := svc.CollectNow("s9"); err != nil {
		t.Fatalf("CollectNow: %v", err)
	}

	if got := <-waited; got.Status != StatusPending {
		t.Fatalf("Wait = %+v, want still pending", got)
	}
	if _, err := svc.CollectNow("s9"); err != nil {
		t.Fatalf("CollectNow: %v", err)
	}
	if err := svc.Reply("s2", "t1", "done"); err != nil {
		t.Fatalf("Reply after the wait = %v, want the ticket still open: a waiter was just told so", err)
	}
}

// An answer that names no ticket belongs to an errand the target has read. A
// message still being pasted, its Enter not sent yet, is not one: the agent
// answering then is answering something else, and attributing it to the
// message mid-delivery sends the sender a report of work nobody did.
func TestATicketlessAnswerSkipsAMessageStillBeingDelivered(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	svc := newRelay(workspace(), term, nil)
	svc.submitDelay = 5 * time.Millisecond
	svc.settleLimit = time.Second
	term.noise("s2", 1000)

	go func() {
		_, _ = svc.Send(context.Background(), SendOptions{From: "s1", Target: "docs", Prompt: "run the tests", WaitSeconds: 1})
	}()
	deadline := time.Now().Add(2 * time.Second)
	for len(term.writesTo("s2")) == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if len(term.writesTo("s2")) != 1 {
		t.Fatalf("writes = %q, want the paste in and the Enter held back", term.writesTo("s2"))
	}

	err := svc.Reply("s2", "", "an answer to something else")
	if err == nil || !strings.Contains(err.Error(), "no open request") {
		t.Errorf("ticketless Reply mid-delivery = %v, want it refused", err)
	}
}

// A wait on an errand that ended without an answer is told so, not that the
// ticket is unknown: the ticket still takes a late answer for an hour, and a
// caller handed "expired" would stop looking for it.
func TestAWaitOnALapsedTicketIsToldItWentUnanswered(t *testing.T) {
	svc := newRelay(workspace(), newFakeTerminal("s1", "s2"), nil)
	sent, _ := svc.SendPrivate(context.Background(), SendOptions{From: "s1", Target: "docs", Prompt: "run the tests", WaitSeconds: 1})
	svc.Observe("s2", "busy")
	svc.Observe("s2", "done")
	if first, err := svc.Wait(context.Background(), sent.Ticket, 1); err != nil || first.Status != StatusUnanswered {
		t.Fatalf("first Wait = %+v, %v, want the stall", first, err)
	}

	got, err := svc.Wait(context.Background(), sent.Ticket, 1)
	if err != nil {
		t.Fatalf("Wait on the lapsed ticket = %v, want it still known", err)
	}
	if got.Status != StatusUnanswered || !got.Private {
		t.Errorf("Wait = %+v, want unanswered so far, still private", got)
	}
}

// A caller hanging up as the turn ends, with the late answer landing in
// between, must not have its leaving overwrite that answer with the stall.
func TestALateAnswerSurvivesTheWaiterHangingUp(t *testing.T) {
	svc := newRelay(workspace(), newFakeTerminal("s1", "s2"), nil)
	tk := &ticket{
		fromID: "s1", targetID: "s2", target: "docs",
		created: time.Now(), delivered: time.Now(), submitted: true,
		done: make(chan struct{}), stalled: make(chan struct{}),
		unread: make(chan struct{}), undelivered: make(chan struct{}),
		attended: 1,
	}
	svc.mu.Lock()
	close(tk.stalled)
	svc.lapseLocked("tk1", tk, StatusUnanswered)
	svc.mu.Unlock()

	if err := svc.Reply("s2", "tk1", "green"); err != nil {
		t.Fatalf("late Reply: %v", err)
	}
	svc.abandon("tk1", tk)

	got, err := svc.Wait(context.Background(), "tk1", 1)
	if err != nil || got.Status != StatusAnswered || got.Answer != "green" {
		t.Errorf("Wait = %+v, %v, want the late answer kept", got, err)
	}
}

// A ticket that ages out unanswered, open or lapsed, answers a wait on it with
// how it ended: "unknown ticket" would read as a ticket the caller mistyped.
func TestAWaitOnAnExpiredTicketSaysItExpired(t *testing.T) {
	svc := newRelay(workspace(), newFakeTerminal("s1", "s2", "s3"), nil)
	plant(svc, "open", "s1", "s2", "docs")
	plant(svc, "lapsed", "s1", "s3", "api")
	svc.mu.Lock()
	svc.lapseLocked("lapsed", svc.tickets["lapsed"], StatusUnanswered)
	delete(svc.tickets, "lapsed")
	svc.mu.Unlock()

	later := time.Now().Add(2 * ticketTTL)
	svc.now = func() time.Time { return later }
	for id, target := range map[string]string{"open": "docs", "lapsed": "api"} {
		got, err := svc.Wait(context.Background(), id, 1)
		if err != nil || got.Status != StatusExpired || got.Target != target {
			t.Errorf("Wait(%s) = %+v, %v, want expired from %q", id, got, err, target)
		}
	}
	if _, err := svc.Wait(context.Background(), "never", 1); err == nil || !strings.Contains(err.Error(), "unknown ticket") {
		t.Errorf("Wait on a ticket that never existed = %v, want unknown ticket", err)
	}
}
