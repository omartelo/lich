package store

import "testing"

func TestSessionSubagentRoundTripsAndSurvivesAResume(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddSession("p1", "base", "Session 1", "claude", "", 2, "")
	_ = svc.AddSession("p1", "wt1", "worker", "claude", "/wt/foo", 3, "")

	if err := svc.SetSessionSubagent("wt1"); err != nil {
		t.Fatalf("SetSessionSubagent: %v", err)
	}
	if !svc.SessionSubagent("wt1") || svc.SessionSubagent("base") || svc.SessionSubagent("ghost") {
		t.Fatal("want only the marked session read as a subagent")
	}
	_ = svc.CloseSession("p1", "wt1", "base")
	if _, err := svc.ReopenWorktreeSession("p1", "/wt/foo", "wt2"); err != nil {
		t.Fatalf("ReopenWorktreeSession: %v", err)
	}
	if !svc.SessionSubagent("wt2") {
		t.Error("the resumed worker lost its subagent mark")
	}
}

func TestSessionBranchReadsTheSessionsCheckout(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddSession("p1", "s1", "Session 1", "claude", "", 2, "")
	_ = svc.AddSession("p1", "wt1", "worker", "claude", "/wt/foo", 3, "")
	if got := svc.SessionBranch("s1"); got != "" {
		t.Errorf("unwired SessionBranch = %q, want empty", got)
	}
	svc.SetBranchOf(func(path string) string { return "branch-of:" + path })

	if got := svc.SessionBranch("wt1"); got != "branch-of:/wt/foo" {
		t.Errorf("SessionBranch(wt1) = %q, want the worktree's", got)
	}
	if got := svc.SessionBranch("s1"); got != "branch-of:/tmp/alpha" {
		t.Errorf("SessionBranch(s1) = %q, want the project directory's", got)
	}
}

// A fork of a subagent worker's conversation is marked a subagent too, found by
// the provider conversation it branches, so the copy keeps its own subagents
// native like the worker it was copied from.
func TestInheritSubagentFollowsTheForkedConversation(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddSession("p1", "worker", "Session 1", "claude", "", 2, "")
	_ = svc.AddSession("p1", "plain", "Session 2", "claude", "", 3, "")
	_ = svc.AddSession("p1", "fork1", "Session 3", "claude", "", 4, "")
	_ = svc.AddSession("p1", "fork2", "Session 4", "claude", "", 5, "")
	_ = svc.SetProviderSession("worker", "conv-worker")
	_ = svc.SetProviderSession("plain", "conv-plain")
	_ = svc.SetSessionSubagent("worker")

	if err := svc.InheritSubagent("fork1", "conv-worker"); err != nil {
		t.Fatalf("InheritSubagent: %v", err)
	}
	if !svc.SessionSubagent("fork1") {
		t.Error("fork of a worker's conversation is not a subagent, want it marked")
	}
	if err := svc.InheritSubagent("fork2", "conv-plain"); err != nil {
		t.Fatalf("InheritSubagent: %v", err)
	}
	if svc.SessionSubagent("fork2") {
		t.Error("fork of a plain conversation is marked a subagent, want it left alone")
	}
}
