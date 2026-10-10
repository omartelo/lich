package relay

import (
	"reflect"
	"testing"
	"time"
)

// plantOwed is an errand from sender's session to targetID whose message went
// in, which is what makes it the target's to answer.
func plantOwed(svc *Service, id, fromID, sender, targetID, asked string) {
	plant(svc, id, fromID, targetID, "docs")
	svc.mu.Lock()
	svc.tickets[id].sender = sender
	svc.tickets[id].asked = asked
	svc.tickets[id].submitted = true
	svc.mu.Unlock()
}

func TestStatusOfASessionWithNoErrandsIsThreeEmptyLists(t *testing.T) {
	svc := newRelay(workspace(), newFakeTerminal("s1"), nil)

	got := svc.Status("s1")

	if got.Owed == nil || got.Open == nil || got.Ready == nil {
		t.Fatalf("Status = %+v, want empty lists, never nil", got)
	}
	if len(got.Owed)+len(got.Open)+len(got.Ready) != 0 {
		t.Fatalf("Status = %+v, want nothing", got)
	}
}

func TestStatusListsWhatTheSessionOwes(t *testing.T) {
	svc := newRelay(workspace(), newFakeTerminal("s1", "s2"), nil)
	plantOwed(svc, "t1", "s1", "sender", "s2", "write the docs")
	plantOwed(svc, "t2", "", "", "s2", "run the tests")
	plant(svc, "t3", "s1", "s2", "docs") // typed in, Enter not through yet

	got := svc.Status("s2").Owed

	want := []OwedErrand{
		{Ticket: "t1", From: "sender", Asked: "write the docs"},
		{Ticket: "t2", From: "", Asked: "run the tests"},
	}
	if !sameOwed(got, want) {
		t.Fatalf("Owed = %+v, want %+v", got, want)
	}
}

// sameOwed compares regardless of order: plant leaves every deliverySeq at
// zero, so the order between two planted errands is not the test's to pin.
func sameOwed(got, want []OwedErrand) bool {
	if len(got) != len(want) {
		return false
	}
	seen := map[OwedErrand]bool{}
	for _, e := range got {
		seen[e] = true
	}
	for _, e := range want {
		if !seen[e] {
			return false
		}
	}
	return true
}

func TestStatusOrdersOwedErrandsByHandOff(t *testing.T) {
	svc := newRelay(workspace(), newFakeTerminal("s1", "s2"), nil)
	plantOwed(svc, "late", "s1", "sender", "s2", "second")
	plantOwed(svc, "early", "s1", "sender", "s2", "first")
	svc.mu.Lock()
	svc.tickets["early"].deliverySeq = 1
	svc.tickets["late"].deliverySeq = 2
	svc.mu.Unlock()

	got := svc.Status("s2").Owed

	if len(got) != 2 || got[0].Ticket != "early" || got[1].Ticket != "late" {
		t.Fatalf("Owed = %+v, want early then late", got)
	}
}

func TestStatusListsOpenErrandsWithTheTargetsState(t *testing.T) {
	svc := newRelay(workspace(), newFakeTerminal("s1", "s2", "s3"), nil)
	base := time.Now()
	plant(svc, "busy", "s1", "s2", "docs")
	plant(svc, "queued", "s1", "s3", "api")
	plantPrivate(svc, "private", "s1", "s2", "docs")
	svc.mu.Lock()
	svc.tickets["busy"].created = base
	svc.tickets["queued"].created = base.Add(time.Second)
	svc.tickets["queued"].delivered = time.Time{}
	svc.mu.Unlock()
	svc.Observe("s2", stateBusy)

	got := svc.Status("s1").Open

	want := []OpenErrand{
		{Ticket: "busy", Target: "docs", State: "busy"},
		{Ticket: "queued", Target: "api", State: "queued"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Open = %+v, want %+v", got, want)
	}
}

func TestStatusReadsTheTargetsWaitingAsWaiting(t *testing.T) {
	svc := newRelay(workspace(), newFakeTerminal("s1", "s2"), nil)
	plant(svc, "t1", "s1", "s2", "docs")
	svc.Observe("s2", stateBusy)
	svc.Observe("s2", stateWaiting)

	got := svc.Status("s1").Open

	if len(got) != 1 || got[0].State != "waiting" {
		t.Fatalf("Open = %+v, want the target waiting", got)
	}
}

// Reading is not collecting: an answer listed as ready is still the agent's to
// collect afterwards.
func TestStatusListsReadyOutcomesWithoutCollectingThem(t *testing.T) {
	svc := newRelay(workspace(), newFakeTerminal("s1", "s2"), nil)
	plant(svc, "t1", "s1", "s2", "docs")
	if err := svc.Reply(ReplyOptions{Ticket: "t1", Answer: "all green"}); err != nil {
		t.Fatalf("Reply: %v", err)
	}

	got := svc.Status("s1")

	want := []ReadyErrand{{Ticket: "t1", Target: "docs", Status: StatusAnswered}}
	if !reflect.DeepEqual(got.Ready, want) || len(got.Open) != 0 {
		t.Fatalf("Status = %+v, want t1 ready and nothing open", got)
	}
	collected, err := svc.CollectNow(CollectNowOptions{From: "s1"})
	if err != nil {
		t.Fatalf("CollectNow: %v", err)
	}
	if len(collected.Results) != 1 || collected.Results[0].Answer != "all green" {
		t.Fatalf("CollectNow after Status = %+v, want the answer still there", collected)
	}
}

func TestStatusLeavesAPrivateOutcomeOut(t *testing.T) {
	svc := newRelay(workspace(), newFakeTerminal("s1", "s2"), nil)
	plantPrivate(svc, "t1", "s1", "s2", "docs")
	if err := svc.Reply(ReplyOptions{Ticket: "t1", Answer: "done"}); err != nil {
		t.Fatalf("Reply: %v", err)
	}

	if got := svc.Status("s1"); len(got.Ready) != 0 || len(got.Open) != 0 {
		t.Fatalf("Status = %+v, want the private errand left out", got)
	}
}

func TestStatusNeverListsAnExpiredErrand(t *testing.T) {
	svc := newRelay(workspace(), newFakeTerminal("s1", "s2"), nil)
	plant(svc, "t1", "s1", "s2", "docs")
	svc.now = func() time.Time { return time.Now().Add(ticketTTL + time.Minute) }

	if got := svc.Status("s1"); len(got.Open) != 0 {
		t.Fatalf("Open = %+v, want the expired errand gone", got.Open)
	}
}
