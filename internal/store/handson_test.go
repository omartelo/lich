package store

import "testing"

// TestAnUnworkedSessionReadsZero pins the starting point: a session nothing has
// been counted for yet has no row, and that has to read as zero rather than as
// an error — every session begins there.
func TestAnUnworkedSessionReadsZero(t *testing.T) {
	svc := newCostSession(t)

	seconds, err := svc.HandsOn("s1")

	if err != nil {
		t.Fatalf("HandsOn: %v", err)
	}
	if seconds != 0 {
		t.Errorf("HandsOn = %d, want 0", seconds)
	}
}

// TestHandsOnAccumulates is the contract the accumulator depends on: it holds
// only the arrears since its last write, so the store has to add rather than
// set — otherwise a restart would restate the session at whatever the next
// flush happened to be carrying.
func TestHandsOnAccumulates(t *testing.T) {
	svc := newCostSession(t)

	for _, seconds := range []int64{30, 30, 45} {
		if err := svc.AddHandsOn("s1", seconds); err != nil {
			t.Fatalf("AddHandsOn: %v", err)
		}
	}

	total, err := svc.HandsOn("s1")
	if err != nil {
		t.Fatalf("HandsOn: %v", err)
	}
	if total != 105 {
		t.Errorf("HandsOn = %d, want 105", total)
	}
}

// TestAddingNothingWritesNothing proves a drain with no whole second in it
// costs no write at all — the debounce fires on a timer, not on there being
// something to say.
func TestAddingNothingWritesNothing(t *testing.T) {
	svc := newCostSession(t)

	if err := svc.AddHandsOn("s1", 0); err != nil {
		t.Fatalf("AddHandsOn: %v", err)
	}

	var rows int
	if err := svc.db.QueryRow(
		`SELECT COUNT(*) FROM session_hands_on WHERE session_id = 's1'`,
	).Scan(&rows); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if rows != 0 {
		t.Errorf("a zero-second add wrote %d rows, want none", rows)
	}
}

// TestAddingToAGoneSessionIsNotAnError pins the same race SaveCostLedger takes:
// a card closed while its last stretch was still being written has no row left
// to add to, and the foreign key must not turn that into a failure.
func TestAddingToAGoneSessionIsNotAnError(t *testing.T) {
	svc := newCostSession(t)

	if err := svc.AddHandsOn("ghost", 60); err != nil {
		t.Errorf("AddHandsOn for a session that is gone: %v", err)
	}
}

// TestDeletingASessionTakesItsHandsOnRow proves the cascade: the row hangs off
// the session and must not outlive it.
func TestDeletingASessionTakesItsHandsOnRow(t *testing.T) {
	svc := newCostSession(t)
	if err := svc.AddHandsOn("s1", 90); err != nil {
		t.Fatalf("AddHandsOn: %v", err)
	}

	if err := svc.DeleteSession("p1", "s1", ""); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}

	var rows int
	if err := svc.db.QueryRow(`SELECT COUNT(*) FROM session_hands_on`).Scan(&rows); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if rows != 0 {
		t.Errorf("%d hands-on rows outlived the session they belong to", rows)
	}
}

// TestResumingAWorktreeSessionKeepsItsHandsOnTime proves the total survives the
// delete-and-reinsert a resume performs. A session parked at lunch and resumed
// after must not come back reading zero: the hours are the user's, and the new
// id is lich's own bookkeeping.
func TestResumingAWorktreeSessionKeepsItsHandsOnTime(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddSession("p1", "base", "Session 1", "claude", "", 2, "")
	_ = svc.AddSession("p1", "wt1", "worktree", "claude", "/wt/foo", 3, "")
	if err := svc.AddHandsOn("wt1", 4200); err != nil {
		t.Fatalf("AddHandsOn: %v", err)
	}
	_ = svc.CloseSession("p1", "wt1", "base")

	if _, err := svc.ReopenWorktreeSession("p1", "/wt/foo", "wt2"); err != nil {
		t.Fatalf("ReopenWorktreeSession: %v", err)
	}

	total, err := svc.HandsOn("wt2")
	if err != nil {
		t.Fatalf("HandsOn: %v", err)
	}
	if total != 4200 {
		t.Errorf("a resumed session reads %ds, want the 4200 it was worked", total)
	}
}
