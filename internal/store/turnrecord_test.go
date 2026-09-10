package store

import "testing"

// The reason the table exists: what a turn recorded has to come back out of a
// database opened by a different run of lich, tree for tree and to the
// millisecond.
func TestTurnRecordRoundTrips(t *testing.T) {
	svc := newCostSession(t)
	want := TurnRecord{Before: "aaa111", After: "bbb222", EndedAt: 1_725_000_000_123}

	if err := svc.SaveTurnRecord("s1", want); err != nil {
		t.Fatalf("SaveTurnRecord: %v", err)
	}
	got, ok, err := svc.TurnRecord("s1")

	if err != nil {
		t.Fatalf("TurnRecord: %v", err)
	}
	if !ok {
		t.Fatal("the saved turn read back as absent")
	}
	if got != want {
		t.Errorf("TurnRecord = %+v, want %+v", got, want)
	}
}

// A session with nothing on record is where every session starts, and the panel
// already has words for it — so it is an absence, never an error.
func TestASessionWithNoTurnReadsAbsent(t *testing.T) {
	svc := newCostSession(t)

	got, ok, err := svc.TurnRecord("s1")

	if err != nil {
		t.Fatalf("TurnRecord: %v", err)
	}
	if ok {
		t.Errorf("an unrecorded session answered %+v", got)
	}
}

// One row per session, replaced by each turn: the panel answers for the LAST
// turn, and a second row would let a later launch pick the wrong one.
func TestSavingAgainReplacesTheTurn(t *testing.T) {
	svc := newCostSession(t)
	if err := svc.SaveTurnRecord("s1", TurnRecord{Before: "old1", After: "old2", EndedAt: 1}); err != nil {
		t.Fatalf("SaveTurnRecord: %v", err)
	}
	want := TurnRecord{Before: "new1", After: "new2", EndedAt: 2}

	if err := svc.SaveTurnRecord("s1", want); err != nil {
		t.Fatalf("SaveTurnRecord: %v", err)
	}

	got, _, err := svc.TurnRecord("s1")
	if err != nil {
		t.Fatalf("TurnRecord: %v", err)
	}
	if got != want {
		t.Errorf("TurnRecord = %+v, want %+v", got, want)
	}
}

// A turn that lost a snapshot has no record. Saving one without both trees
// clears the row rather than leaving the turn before it standing, which is the
// same rule the in-memory accounting follows (internal/terminal.closeTurn).
func TestATurnWithoutBothTreesClearsTheRow(t *testing.T) {
	svc := newCostSession(t)
	if err := svc.SaveTurnRecord("s1", TurnRecord{Before: "aaa", After: "bbb", EndedAt: 1}); err != nil {
		t.Fatalf("SaveTurnRecord: %v", err)
	}

	if err := svc.SaveTurnRecord("s1", TurnRecord{}); err != nil {
		t.Fatalf("SaveTurnRecord: %v", err)
	}

	got, ok, err := svc.TurnRecord("s1")
	if err != nil {
		t.Fatalf("TurnRecord: %v", err)
	}
	if ok {
		t.Errorf("the retracted turn survived as %+v", got)
	}
}

// The closing snapshot lands on the worker long after the `done` that asked for
// it, so the card can be gone by then. That is a race, not a failure — and the
// foreign key would otherwise turn it into one.
func TestSavingForAGoneSessionIsNotAnError(t *testing.T) {
	svc := newCostSession(t)

	if err := svc.SaveTurnRecord("never-existed", TurnRecord{Before: "a", After: "b"}); err != nil {
		t.Fatalf("SaveTurnRecord: %v", err)
	}

	if _, ok, _ := svc.TurnRecord("never-existed"); ok {
		t.Error("a turn was filed for a session with no row")
	}
}

// The record belongs to the card: deleting the session takes it with it, so a
// later session reusing the id can never inherit it.
func TestDeletingTheSessionDropsItsTurn(t *testing.T) {
	svc := newCostSession(t)
	if err := svc.SaveTurnRecord("s1", TurnRecord{Before: "aaa", After: "bbb", EndedAt: 1}); err != nil {
		t.Fatalf("SaveTurnRecord: %v", err)
	}

	if err := svc.DeleteSession("p1", "s1", ""); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}

	if _, ok, err := svc.TurnRecord("s1"); err != nil || ok {
		t.Errorf("the turn outlived its session (ok=%v, err=%v)", ok, err)
	}
}

// The Review panel draws its source switch off the hydration, long before
// anything asks what the turn changed: a restored session has to say it holds a
// record, or the switch that reaches it is withheld until it next reports.
func TestHydrationSaysWhichSessionsHoldATurn(t *testing.T) {
	svc := newCostSession(t)
	if err := svc.AddSession("p1", "s2", "Session 2", "", "", 2, ""); err != nil {
		t.Fatalf("AddSession: %v", err)
	}
	if err := svc.SaveTurnRecord("s1", TurnRecord{Before: "aaa", After: "bbb", EndedAt: 1}); err != nil {
		t.Fatalf("SaveTurnRecord: %v", err)
	}

	sessions, err := svc.sessionsOf("p1")
	if err != nil {
		t.Fatalf("sessionsOf: %v", err)
	}

	held := map[string]bool{}
	for _, sess := range sessions {
		held[sess.ID] = sess.HasLastTurn
	}
	if !held["s1"] {
		t.Error("the session holding a turn hydrated without it")
	}
	if held["s2"] {
		t.Error("a session with no turn on record claimed one")
	}
}
