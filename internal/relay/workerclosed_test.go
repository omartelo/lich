package relay

import (
	"context"
	"sync"
	"testing"
	"time"
)

// Closing a worker is its caller stopping it (TaskStop runs `lich close`), so
// the errand ends there and the caller is told nothing: the SessionEnd its
// dying CLI reports a moment later finds no errand to call unanswered.
func TestClosingAWorkerEndsItsErrandSilently(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	events := &fakeEvents{}
	svc := newRelay(workspace(), term, events)
	plantSubagent(svc, "t1", "s1", "s2", "docs")

	svc.SessionClosed("s2", "s1")
	svc.Observe("s2", stateIdle)
	time.Sleep(20 * time.Millisecond)

	if typed := term.written("s1"); typed != "" {
		t.Errorf("the caller was told %q", typed)
	}
	if len(events.stalled()) != 0 {
		t.Errorf("stalled events = %+v, want none", events.stalled())
	}
	collected, err := svc.CollectNow(CollectNowOptions{From: "s1"})
	if err != nil {
		t.Fatalf("CollectNow: %v", err)
	}
	if len(collected.Results) != 0 || len(collected.Open) != 0 {
		t.Errorf("collect = %+v, want nothing waiting and nothing open", collected)
	}
	if mark, ok := events.markOf("s1"); ok && mark.Ticket != "" {
		t.Errorf("the caller's card still carries %+v", mark)
	}
}

// A worker closed by someone other than its caller, from its card in the window
// or by another session, ends work the caller never stopped, so the caller hears
// it stopped the way it hears any other outcome: in its inbox, with a nudge.
func TestAWorkerClosedByAnotherHandTellsItsCallerItStopped(t *testing.T) {
	for _, closer := range []string{"", "s3"} {
		t.Run("closer="+closer, func(t *testing.T) {
			term := newFakeTerminal("s1", "s2", "s3")
			svc := newRelay(workspace(), term, nil)
			plantSubagent(svc, "t1", "s1", "s2", "docs")

			svc.SessionClosed("s2", closer)

			if !awaitWritten(term, "s1", "[lich]") {
				t.Fatal("the caller was never told its worker stopped")
			}
			collected, err := svc.CollectNow(CollectNowOptions{From: "s1"})
			if err != nil {
				t.Fatalf("CollectNow: %v", err)
			}
			if len(collected.Results) != 1 || collected.Results[0].Status != StatusStopped {
				t.Errorf("collect = %+v, want the errand stopped", collected)
			}
		})
	}
}

// A caller still holding the line hears how the errand ended: stopped, which no
// card has to be opened about.
func TestAWaiterOnAClosedWorkerHearsItStopped(t *testing.T) {
	svc := newRelay(workspace(), newFakeTerminal("s1", "s2"), nil)
	plantSubagent(svc, "t1", "s1", "s2", "docs")

	var (
		got Result
		wg  sync.WaitGroup
	)
	svc.mu.Lock()
	svc.tickets["t1"].attended++
	tk := svc.tickets["t1"]
	svc.mu.Unlock()
	wg.Go(func() { got = svc.await(context.Background(), "t1", tk, time.Second) })
	svc.SessionClosed("s2", "s1")
	wg.Wait()

	if got.Status != StatusStopped {
		t.Errorf("status = %q, want %q", got.Status, StatusStopped)
	}
	again, err := svc.Wait(context.Background(), WaitOptions{Ticket: "t1", WaitSeconds: 1})
	if err != nil || again.Status != StatusStopped {
		t.Errorf("a later Wait = %+v, %v, want it stopped", again, err)
	}
}

// Only subagent errands end with their worker's close: an ordinary errand at
// a closed session still ends as it always did.
func TestClosingASessionLeavesOrdinaryErrandsToTheirEnd(t *testing.T) {
	svc := newRelay(workspace(), newFakeTerminal("s1", "s2"), nil)
	plant(svc, "t1", "s1", "s2", "docs")

	svc.SessionClosed("s2", "")

	svc.mu.Lock()
	defer svc.mu.Unlock()
	if svc.tickets["t1"] == nil {
		t.Error("an ordinary errand was ended by its target's close")
	}
}
