package relay

import (
	"context"
	"sync"
	"testing"
	"time"
)

// gatedEvents holds the first mark cleared on gate's card until release is
// closed. Reply clears the marks between closing a ticket and filing its
// outcome, so holding that event parks the answer exactly in the gap a Wait
// must never fall into.
type gatedEvents struct {
	gate    string
	once    sync.Once
	reached chan struct{}
	release chan struct{}
}

func (g *gatedEvents) Emit(_ string, data any) {
	event, ok := data.(RelayEvent)
	if !ok || event.ID != g.gate || event.Peer != "" {
		return
	}
	g.once.Do(func() {
		close(g.reached)
		<-g.release
	})
}

// A Wait that lands while an answer is being handed over must find it, in the
// ticket or in the inbox. Closing the ticket and filing its outcome used to be
// two critical sections apart, and a Wait between them found neither: "unknown
// ticket" for an errand that had just been answered.
func TestAWaitAsTheAnswerLandsNeverSeesAnUnknownTicket(t *testing.T) {
	events := &gatedEvents{gate: "s2", reached: make(chan struct{}), release: make(chan struct{})}
	svc := newRelay(workspace(), newFakeTerminal("s1", "s2"), events)
	plant(svc, "t1", "s1", "s2", "docs")

	replied := make(chan error, 1)
	go func() { replied <- svc.Reply(ReplyOptions{Ticket: "t1", Answer: "done"}) }()
	<-events.reached

	got, err := svc.Wait(context.Background(), WaitOptions{Ticket: "t1", WaitSeconds: 1})
	close(events.release)
	if replyErr := <-replied; replyErr != nil {
		t.Fatalf("Reply: %v", replyErr)
	}
	if err != nil {
		t.Fatalf("Wait = %v, want the answer that was landing", err)
	}
	if got.Status != StatusAnswered || got.Answer != "done" {
		t.Fatalf("Wait = %+v, want answered with %q", got, "done")
	}
}

// An answer is carried out once. A Wait used to count itself as attending only
// after leaving the lock it found the ticket under, so a Reply in between filed
// the answer for an absent sender while the Wait also returned it: the inbox
// then announced a result its caller already had.
//
// The gap is reached through the expiry Wait sweeps on its way in: an old
// ticket's marks are cleared after the lock is let go and before await runs,
// and holding that event parks the Wait there while the answer lands.
func TestAnAnswerAWaitCarriedOutIsNotAlsoFiled(t *testing.T) {
	events := &gatedEvents{gate: "s3", reached: make(chan struct{}), release: make(chan struct{})}
	svc := newRelay(workspace(), newFakeTerminal("s1", "s2", "s3"), events)
	plant(svc, "t1", "s1", "s2", "docs")
	plant(svc, "old", "s1", "s3", "api")
	svc.mu.Lock()
	svc.tickets["old"].created = time.Now().Add(-2 * ticketTTL)
	svc.mu.Unlock()

	waited := make(chan Result, 1)
	go func() {
		got, err := svc.Wait(context.Background(), WaitOptions{Ticket: "t1", WaitSeconds: 1})
		if err != nil {
			t.Errorf("Wait: %v", err)
		}
		waited <- got
	}()
	<-events.reached
	if err := svc.Reply(ReplyOptions{Ticket: "t1", Answer: "done"}); err != nil {
		t.Fatalf("Reply: %v", err)
	}
	close(events.release)

	if got := <-waited; got.Status != StatusAnswered || got.Answer != "done" {
		t.Fatalf("Wait = %+v, want answered with %q", got, "done")
	}
	svc.mu.Lock()
	left := len(svc.ready)
	svc.mu.Unlock()
	if left != 0 {
		t.Fatalf("the Wait returned the answer and %d result(s) are still filed for it", left)
	}
}
