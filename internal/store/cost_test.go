package store

import "testing"

// newCostSession opens a store with one project and one session to bill.
func newCostSession(t *testing.T) *Service {
	t.Helper()
	svc := newTestStore(t)
	if err := svc.AddProject("p1", "alpha", "/tmp/alpha"); err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	if err := svc.AddSession("p1", "s1", "Session 1", "", "", 2, ""); err != nil {
		t.Fatalf("AddSession: %v", err)
	}
	return svc
}

// TestAnUncountedTranscriptStartsAtZero pins the starting point every session
// begins from: no row yet means offset 0, so the first scan reads the whole
// transcript instead of erroring.
func TestAnUncountedTranscriptStartsAtZero(t *testing.T) {
	svc := newCostSession(t)

	offset, lastMessage, cost, err := svc.CostLedger("s1", "uuid-a")

	if err != nil {
		t.Fatalf("CostLedger: %v", err)
	}
	if offset != 0 || lastMessage != "" || cost != 0 {
		t.Errorf("ledger = (%d, %q, %v), want the zero starting point", offset, lastMessage, cost)
	}
}

// TestTheLedgerRoundTrips proves a saved position comes back intact — it is
// what makes the next scan resume rather than recount.
func TestTheLedgerRoundTrips(t *testing.T) {
	svc := newCostSession(t)

	if err := svc.SaveCostLedger("s1", "uuid-a", 512, "msg-7", 0.25); err != nil {
		t.Fatalf("SaveCostLedger: %v", err)
	}
	if err := svc.SaveCostLedger("s1", "uuid-a", 1024, "msg-9", 0.5); err != nil {
		t.Fatalf("SaveCostLedger (update): %v", err)
	}

	offset, lastMessage, cost, err := svc.CostLedger("s1", "uuid-a")
	if err != nil {
		t.Fatalf("CostLedger: %v", err)
	}
	if offset != 1024 || lastMessage != "msg-9" || cost != 0.5 {
		t.Errorf("ledger = (%d, %q, %v), want the second save", offset, lastMessage, cost)
	}
}

// TestSessionCostSumsEveryConversation is the `/clear` contract at the storage
// layer: each conversation keeps its own row, and the session's number is all
// of them.
func TestSessionCostSumsEveryConversation(t *testing.T) {
	svc := newCostSession(t)
	if err := svc.SaveCostLedger("s1", "uuid-a", 100, "m1", 0.25); err != nil {
		t.Fatalf("SaveCostLedger: %v", err)
	}
	if err := svc.SaveCostLedger("s1", "uuid-b", 100, "m2", 0.75); err != nil {
		t.Fatalf("SaveCostLedger: %v", err)
	}

	cost, err := svc.SessionCost("s1")

	if err != nil {
		t.Fatalf("SessionCost: %v", err)
	}
	if cost != 1.0 {
		t.Errorf("SessionCost = %v, want 1.0", cost)
	}
}

// TestASessionWithNothingCountedCostsZero pins the empty read: SUM over no rows
// is NULL in SQLite, and a NULL that reached the caller as an error would take
// the readout down for every session's first turn.
func TestASessionWithNothingCountedCostsZero(t *testing.T) {
	svc := newCostSession(t)

	cost, err := svc.SessionCost("s1")

	if err != nil {
		t.Fatalf("SessionCost: %v", err)
	}
	if cost != 0 {
		t.Errorf("SessionCost = %v, want 0", cost)
	}
}

// TestClosingASessionForgetsItsCost proves the ledgers go with the session
// they belong to — the workspace never accumulates rows for sessions that are
// gone.
func TestClosingASessionForgetsItsCost(t *testing.T) {
	svc := newCostSession(t)
	if err := svc.SaveCostLedger("s1", "uuid-a", 100, "m1", 0.25); err != nil {
		t.Fatalf("SaveCostLedger: %v", err)
	}

	if err := svc.DeleteSession("p1", "s1", ""); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}

	cost, err := svc.SessionCost("s1")
	if err != nil {
		t.Fatalf("SessionCost: %v", err)
	}
	if cost != 0 {
		t.Errorf("SessionCost = %v, want 0 after the session was deleted", cost)
	}
}

// TestALedgerForAMissingSessionIsDropped pins the race a turn can lose: the
// session is closed while its last turn is still being counted. The write finds
// nothing to attach to and says nothing, rather than failing the readout.
func TestALedgerForAMissingSessionIsDropped(t *testing.T) {
	svc := newCostSession(t)

	if err := svc.SaveCostLedger("gone", "uuid-a", 100, "m1", 0.25); err != nil {
		t.Fatalf("SaveCostLedger: %v", err)
	}

	cost, err := svc.SessionCost("gone")
	if err != nil {
		t.Fatalf("SessionCost: %v", err)
	}
	if cost != 0 {
		t.Errorf("SessionCost = %v, want 0 — nothing should have been written", cost)
	}
}

// TestAResumedSessionKeepsWhatItSpent covers the park-and-reopen path, which
// re-keys the session under a fresh id. The conversation continues, so its
// total must too.
func TestAResumedSessionKeepsWhatItSpent(t *testing.T) {
	svc := newTestStore(t)
	if err := svc.AddProject("p1", "alpha", "/tmp/alpha"); err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	if err := svc.AddSession("p1", "s1", "Session 1", "claude", "/tmp/alpha-wt", 2, ""); err != nil {
		t.Fatalf("AddSession: %v", err)
	}
	if err := svc.SaveCostLedger("s1", "uuid-a", 100, "m1", 0.4); err != nil {
		t.Fatalf("SaveCostLedger: %v", err)
	}
	if err := svc.CloseSession("p1", "s1", ""); err != nil {
		t.Fatalf("CloseSession: %v", err)
	}

	restored, err := svc.ReopenWorktreeSession("p1", "/tmp/alpha-wt", "s2")
	if err != nil {
		t.Fatalf("ReopenWorktreeSession: %v", err)
	}
	if restored == nil {
		t.Fatal("ReopenWorktreeSession: want the parked session back")
	}

	cost, err := svc.SessionCost("s2")
	if err != nil {
		t.Fatalf("SessionCost: %v", err)
	}
	if cost != 0.4 {
		t.Errorf("SessionCost = %v, want 0.4 carried onto the resumed session", cost)
	}
}

// TestCostReadoutIsOffUntilAskedFor is the feature's premise at the setting
// layer: the number only means something on API billing, so it is off for
// everyone who never turned it on, and anything but "true" reads as off.
func TestCostReadoutIsOffUntilAskedFor(t *testing.T) {
	svc := newTestStore(t)

	if svc.CostReadout() {
		t.Error("CostReadout = true on a fresh workspace, want off")
	}

	for _, value := range []string{"1", "yes", "", "TRUE"} {
		if err := svc.SetSetting("usage.cost", "", value); err != nil {
			t.Fatalf("SetSetting: %v", err)
		}
		if svc.CostReadout() {
			t.Errorf("CostReadout = true for %q, want off", value)
		}
	}

	if err := svc.SetSetting("usage.cost", "", "true"); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	if !svc.CostReadout() {
		t.Error("CostReadout = false after it was turned on")
	}
}

// TestAForkIsNotBilledForTheHistoryItInherits is the whole point of the offset:
// the copy's own transcript carries the parent's turns, so the ledger counts
// them again and the session readout takes them back off. The parent is left
// alone — it spent that money once and still reports it.
func TestAForkIsNotBilledForTheHistoryItInherits(t *testing.T) {
	svc := newCostSession(t)
	if err := svc.SaveCostLedger("s1", "uuid-a", 100, "m1", 1.25); err != nil {
		t.Fatalf("SaveCostLedger: %v", err)
	}
	if err := svc.AddSession("p1", "s2", "Session 2", "", "", 3, ""); err != nil {
		t.Fatalf("AddSession: %v", err)
	}
	if err := svc.SaveForkCostOffset("s2", "uuid-a"); err != nil {
		t.Fatalf("SaveForkCostOffset: %v", err)
	}
	// The fork's own transcript: the parent's 1.25 of history, plus 0.5 of its
	// own.
	if err := svc.SaveCostLedger("s2", "uuid-b", 200, "m2", 1.75); err != nil {
		t.Fatalf("SaveCostLedger: %v", err)
	}

	fork, err := svc.SessionCost("s2")
	if err != nil {
		t.Fatalf("SessionCost: %v", err)
	}
	parent, err := svc.SessionCost("s1")
	if err != nil {
		t.Fatalf("SessionCost: %v", err)
	}

	if fork != 0.5 {
		t.Errorf("fork cost = %v, want the 0.5 it spent itself", fork)
	}
	if parent != 1.25 {
		t.Errorf("parent cost = %v, want its own 1.25 untouched", parent)
	}
}

// TestAForkOfAForkOffsetsItsOwnParent: the offset is the parent's raw ledger
// row, history included, and never its netted readout — which is what keeps a
// chain of forks adding up to one conversation's spend instead of subtracting
// the same history twice.
func TestAForkOfAForkOffsetsItsOwnParent(t *testing.T) {
	svc := newCostSession(t)
	if err := svc.SaveCostLedger("s1", "uuid-a", 100, "m1", 1.25); err != nil {
		t.Fatalf("SaveCostLedger: %v", err)
	}
	for _, id := range []string{"s2", "s3"} {
		if err := svc.AddSession("p1", id, id, "", "", 3, ""); err != nil {
			t.Fatalf("AddSession: %v", err)
		}
	}
	// s2 forks s1's conversation and spends 0.5 of its own on top of it.
	if err := svc.SaveForkCostOffset("s2", "uuid-a"); err != nil {
		t.Fatalf("SaveForkCostOffset: %v", err)
	}
	if err := svc.SaveCostLedger("s2", "uuid-b", 200, "m2", 1.75); err != nil {
		t.Fatalf("SaveCostLedger: %v", err)
	}
	// s3 forks s2's, and spends 0.25.
	if err := svc.SaveForkCostOffset("s3", "uuid-b"); err != nil {
		t.Fatalf("SaveForkCostOffset: %v", err)
	}
	if err := svc.SaveCostLedger("s3", "uuid-c", 300, "m3", 2.0); err != nil {
		t.Fatalf("SaveCostLedger: %v", err)
	}

	cost, err := svc.SessionCost("s3")

	if err != nil {
		t.Fatalf("SessionCost: %v", err)
	}
	if cost != 0.25 {
		t.Errorf("fork of a fork = %v, want the 0.25 it spent itself", cost)
	}
}

// TestAForkNeverReportsMoneyBack: the offset is a snapshot of a conversation the
// fork's own ledger has not caught up with yet, and a negative dollar figure on
// a card is worse than a low one.
func TestAForkNeverReportsMoneyBack(t *testing.T) {
	svc := newCostSession(t)
	if err := svc.SaveCostLedger("s1", "uuid-a", 100, "m1", 1.25); err != nil {
		t.Fatalf("SaveCostLedger: %v", err)
	}
	if err := svc.AddSession("p1", "s2", "Session 2", "", "", 3, ""); err != nil {
		t.Fatalf("AddSession: %v", err)
	}
	if err := svc.SaveForkCostOffset("s2", "uuid-a"); err != nil {
		t.Fatalf("SaveForkCostOffset: %v", err)
	}

	cost, err := svc.SessionCost("s2")

	if err != nil {
		t.Fatalf("SessionCost: %v", err)
	}
	if cost != 0 {
		t.Errorf("SessionCost = %v, want 0 rather than a negative total", cost)
	}
}

// TestForkingAnUncountedConversationOffsetsNothing: a conversation lich never
// priced — the readout off while it ran, a model with no rate — has no money to
// give back, and inventing one would bill the fork below what it spent.
func TestForkingAnUncountedConversationOffsetsNothing(t *testing.T) {
	svc := newCostSession(t)
	if err := svc.SaveForkCostOffset("s1", "uuid-never-counted"); err != nil {
		t.Fatalf("SaveForkCostOffset: %v", err)
	}
	if err := svc.SaveCostLedger("s1", "uuid-b", 200, "m2", 0.75); err != nil {
		t.Fatalf("SaveCostLedger: %v", err)
	}

	cost, err := svc.SessionCost("s1")

	if err != nil {
		t.Fatalf("SessionCost: %v", err)
	}
	if cost != 0.75 {
		t.Errorf("SessionCost = %v, want the whole 0.75 counted", cost)
	}
}
