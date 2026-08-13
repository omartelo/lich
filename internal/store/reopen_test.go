package store

import "testing"

// TestReopenWorktreeSessionTakesTheNewestParkedRow pins which row a resume
// consumes when a worktree has been parked more than once — a keep-the-worktree
// close after an earlier resume leaves both rows behind. The newest is the
// conversation the user was actually in; resuming the older one would continue a
// transcript they left turns ago, and it is the one row the query orders for.
func TestReopenWorktreeSessionTakesTheNewestParkedRow(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddSession("p1", "base", "Session 1", "claude", "", 2)

	_ = svc.AddSession("p1", "wt1", "older", "claude", "/wt/foo", 3)
	_ = svc.SetProviderSession("wt1", "conv-older")
	_ = svc.CloseSession("p1", "wt1", "base")
	_ = svc.AddSession("p1", "wt2", "newer", "claude", "/wt/foo", 4)
	_ = svc.SetProviderSession("wt2", "conv-newer")
	_ = svc.CloseSession("p1", "wt2", "base")

	restored, err := svc.ReopenWorktreeSession("p1", "/wt/foo", "wt3")
	if err != nil {
		t.Fatalf("ReopenWorktreeSession: %v", err)
	}
	if restored == nil {
		t.Fatal("ReopenWorktreeSession = nil, want the parked session")
	}
	if restored.Label != "newer" || restored.ProviderSessionID != "conv-newer" {
		t.Errorf("restored = %+v, want the newest park (newer/conv-newer)", restored)
	}

	// Only one park is consumed: the older row is still there for its own
	// resume, and the resumed one is not duplicated.
	var parked int
	if err := svc.db.QueryRow(
		`SELECT COUNT(*) FROM sessions WHERE path = ? AND is_open = 0`, "/wt/foo",
	).Scan(&parked); err != nil {
		t.Fatalf("count parked: %v", err)
	}
	if parked != 1 {
		t.Errorf("parked rows after resume = %d, want the older one alone", parked)
	}
}
