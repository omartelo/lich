package store

import "testing"

// TestSessionOriginPersistsAndDefaults pins the two halves of the origin: the id
// of the session that asked for this one, and the label it was called at the
// time. Both come back from LoadState, and a session nobody delegated carries
// neither — absent is the normal case.
func TestSessionOriginPersistsAndDefaults(t *testing.T) {
	svc := newTestStore(t)
	if err := svc.AddProject("p1", "alpha", "/tmp/alpha"); err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	if err := svc.AddSession("p1", "parent", "planner", "claude", "", 2, ""); err != nil {
		t.Fatalf("AddSession: %v", err)
	}
	if err := svc.AddSessionFrom("p1", "child", "worker", "claude", "/wt/a", 3, "parent", "planner"); err != nil {
		t.Fatalf("AddSessionFrom: %v", err)
	}

	projects, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if len(projects) != 1 || len(projects[0].Sessions) != 2 {
		t.Fatalf("state = %+v", projects)
	}
	if parent := projects[0].Sessions[0]; parent.OriginSessionID != "" || parent.OriginLabel != "" {
		t.Errorf("parent origin = %+v, want none", parent)
	}
	child := projects[0].Sessions[1]
	if child.OriginSessionID != "parent" || child.OriginLabel != "planner" {
		t.Errorf("child origin = (%q, %q), want (parent, planner)", child.OriginSessionID, child.OriginLabel)
	}
}

// TestSessionOriginSurvivesARenameOfTheParent proves the shape does what a
// single stored name could not: the id is what carries the rename, so the row
// keeps naming the parent no matter what the parent is called now. The label
// stored beside it is the fallback for a parent that is gone, and is deliberately
// the name the delegation happened under.
func TestSessionOriginSurvivesARenameOfTheParent(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddSession("p1", "parent", "planner", "claude", "", 2, "")
	_ = svc.AddSessionFrom("p1", "child", "worker", "claude", "", 3, "parent", "planner")

	if err := svc.RenameSession("parent", "the boss"); err != nil {
		t.Fatalf("RenameSession: %v", err)
	}
	projects, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	sessions := projects[0].Sessions
	if sessions[0].Label != "the boss" {
		t.Errorf("parent label = %q, want the boss", sessions[0].Label)
	}
	if sessions[1].OriginSessionID != "parent" {
		t.Errorf("child origin id = %q, want it to still name the parent", sessions[1].OriginSessionID)
	}
}

// TestReopenWorktreeSessionCarriesTheOrigin proves a parked worktree session
// comes back knowing where it came from. The resume reinserts the row under a
// fresh id, so anything not copied across is lost — and the origin is identity,
// which must outlive a park exactly as the label and the conversation id do.
func TestReopenWorktreeSessionCarriesTheOrigin(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddSession("p1", "parent", "planner", "claude", "", 2, "")
	_ = svc.AddSessionFrom("p1", "child", "worker", "claude", "/wt/a", 3, "parent", "planner")
	_ = svc.CloseSession("p1", "child", "parent")

	restored, err := svc.ReopenWorktreeSession("p1", "/wt/a", "child2")
	if err != nil {
		t.Fatalf("ReopenWorktreeSession: %v", err)
	}
	if restored == nil {
		t.Fatal("ReopenWorktreeSession = nil, want the parked session")
	}
	if restored.OriginSessionID != "parent" || restored.OriginLabel != "planner" {
		t.Errorf("restored origin = (%q, %q), want (parent, planner)",
			restored.OriginSessionID, restored.OriginLabel)
	}
	projects, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	var reloaded Session
	for _, s := range projects[0].Sessions {
		if s.ID == "child2" {
			reloaded = s
		}
	}
	if reloaded.OriginSessionID != "parent" || reloaded.OriginLabel != "planner" {
		t.Errorf("reloaded origin = %+v, want the parent it was opened from", reloaded)
	}
}
