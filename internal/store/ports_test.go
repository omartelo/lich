package store

import "testing"

// TestAReservedPortRoundTrips proves the reservation survives the write, which
// is the whole reason the table exists: the allocator has to be able to see
// which numbers are spoken for after a restart of lich.
func TestAReservedPortRoundTrips(t *testing.T) {
	svc := newTestStore(t)
	dir := t.TempDir()

	if err := svc.SetWorktreePort(dir, 24007); err != nil {
		t.Fatalf("SetWorktreePort: %v", err)
	}

	if got := svc.WorktreePorts()[dir]; got != 24007 {
		t.Errorf("WorktreePorts()[%q] = %d, want 24007", dir, got)
	}
}

// TestReservingTwiceReplacesTheNumber proves a checkout holds one port, not a
// history of them — otherwise re-reserving would fail on the primary key and
// leave the stale number in the table.
func TestReservingTwiceReplacesTheNumber(t *testing.T) {
	svc := newTestStore(t)
	dir := t.TempDir()

	if err := svc.SetWorktreePort(dir, 24007); err != nil {
		t.Fatalf("SetWorktreePort: %v", err)
	}
	if err := svc.SetWorktreePort(dir, 24008); err != nil {
		t.Fatalf("SetWorktreePort again: %v", err)
	}

	if got := svc.WorktreePorts()[dir]; got != 24008 {
		t.Errorf("WorktreePorts()[%q] = %d, want the replacement 24008", dir, got)
	}
}

// TestAVanishedCheckoutReleasesItsPort proves the only release this table has:
// a reservation whose directory is gone stops being reported, so the number
// goes back to the pool instead of being held forever by a removed worktree.
func TestAVanishedCheckoutReleasesItsPort(t *testing.T) {
	svc := newTestStore(t)

	if err := svc.SetWorktreePort("/definitely/not/a/checkout", 24007); err != nil {
		t.Fatalf("SetWorktreePort: %v", err)
	}

	if ports := svc.WorktreePorts(); len(ports) != 0 {
		t.Errorf("WorktreePorts() = %v, want the vanished checkout dropped", ports)
	}
}

// TestAVanishedCheckoutIsPurgedFromTheTable proves the drop above is a delete
// and not a filter: a row that outlives its directory would otherwise be
// re-read on every allocation for as long as the database lives.
func TestAVanishedCheckoutIsPurgedFromTheTable(t *testing.T) {
	svc := newTestStore(t)

	if err := svc.SetWorktreePort("/definitely/not/a/checkout", 24007); err != nil {
		t.Fatalf("SetWorktreePort: %v", err)
	}
	svc.WorktreePorts()

	var rows int
	if err := svc.db.QueryRow(`SELECT count(*) FROM worktree_ports`).Scan(&rows); err != nil {
		t.Fatalf("count worktree_ports: %v", err)
	}
	if rows != 0 {
		t.Errorf("worktree_ports holds %d rows, want the vanished checkout purged", rows)
	}
}

// TestALiveCheckoutSurvivesThePurge proves the purge is aimed at gone
// directories only — a reservation dropped while its worktree still exists
// would hand the same port to a second checkout.
func TestALiveCheckoutSurvivesThePurge(t *testing.T) {
	svc := newTestStore(t)
	live, gone := t.TempDir(), "/definitely/not/a/checkout"

	if err := svc.SetWorktreePort(live, 24007); err != nil {
		t.Fatalf("SetWorktreePort: %v", err)
	}
	if err := svc.SetWorktreePort(gone, 24008); err != nil {
		t.Fatalf("SetWorktreePort: %v", err)
	}

	ports := svc.WorktreePorts()
	if len(ports) != 1 || ports[live] != 24007 {
		t.Errorf("WorktreePorts() = %v, want only %q at 24007", ports, live)
	}
}
