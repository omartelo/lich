package store

import "testing"

// newCostSession opens a store with one project and one session to bill.
func newCostSession(t *testing.T) *Service {
	t.Helper()
	svc := newTestStore(t)
	if err := svc.AddProject("p1", "alpha", "/tmp/alpha"); err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	if err := svc.AddSession("p1", "s1", "Session 1", "", "", 2); err != nil {
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
	if err := svc.AddSession("p1", "s1", "Session 1", "claude", "/tmp/alpha-wt", 2); err != nil {
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
