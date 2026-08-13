package spawn

import (
	"testing"

	"github.com/omartelo/lich/internal/project"
)

func TestWorktreesReportsWhatIsInEachCheckout(t *testing.T) {
	svc, _, worktrees, _, _ := closer(t)
	worktrees.dirty["/wt/alone"] = true
	// git lists the repository itself; it is not a checkout anything here can
	// act on, so it must not reach the caller.
	worktrees.checkouts = append(
		[]project.Worktree{{Name: "main", Path: "/src/lich"}}, worktrees.checkouts...,
	)

	found, err := svc.Worktrees("s1", "")
	if err != nil {
		t.Fatalf("Worktrees: %v", err)
	}
	if len(found) != 2 {
		t.Fatalf("got %d checkouts, want 2", len(found))
	}

	// The sessions are what make the list actionable: a checkout with one
	// session is a checkout whose fate that session's close decides.
	shared, alone := found[0], found[1]
	if shared.Name != "shared" || len(shared.Sessions) != 2 {
		t.Errorf("shared = %+v, want both its sessions named", shared)
	}
	if shared.Dirty {
		t.Errorf("shared reported as dirty")
	}
	if alone.Name != "alone" || !alone.Dirty {
		t.Errorf("alone = %+v, want the uncommitted work reported", alone)
	}
	if len(alone.Sessions) != 1 || alone.Sessions[0] != "alone" {
		t.Errorf("alone sessions = %v", alone.Sessions)
	}
}

func TestWorktreesNamesAnEmptyCheckoutAsEmpty(t *testing.T) {
	svc, _, worktrees, _, _ := closer(t)
	worktrees.checkouts = []project.Worktree{{Name: "parked", Path: "/wt/parked"}}

	found, err := svc.Worktrees("s1", "")
	if err != nil {
		t.Fatalf("Worktrees: %v", err)
	}
	// Empty, never null: a caller reading this as JSON should not have to tell
	// "no sessions" apart from "no answer".
	if len(found) != 1 || found[0].Sessions == nil || len(found[0].Sessions) != 0 {
		t.Errorf("found = %+v, want one checkout with an empty session list", found)
	}
}

func TestWorktreesRefusesAProjectItCannotResolve(t *testing.T) {
	svc, _, _, _, _ := closer(t)

	if _, err := svc.Worktrees("", ""); err == nil {
		t.Fatal("listed the worktrees of no project in particular")
	}
}

// A session asking for a branch that is already checked out is asking to work
// on that branch: the checkout is opened, not created a second time.
func TestOpenUsesACheckoutThatIsAlreadyThere(t *testing.T) {
	svc, sessions, worktrees, term, _ := closer(t)

	opened, err := svc.Open("s1", "", "", "shared", "", "")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if len(worktrees.created) != 0 {
		t.Errorf("created a worktree that was already there: %+v", worktrees.created)
	}
	if opened.Path != "/wt/shared" || term.spawns[0].cwd != "/wt/shared" {
		t.Errorf("opened = %+v, spawn cwd = %q; want the existing checkout", opened, term.spawns[0].cwd)
	}
	// The setup script belongs to a fresh checkout. This one ran it when it was
	// made, and running it again would reinstall over a working tree.
	if term.spawns[0].setup {
		t.Error("ran the setup script again in a checkout that already exists")
	}
	// Nothing in the project answers to "shared" — the sessions there are
	// "shared-a" and "shared-b" — so the card takes the checkout's own name.
	if opened.Label != "shared" {
		t.Errorf("label = %q, want the checkout's name", opened.Label)
	}
	if len(sessions.rows) != 1 || sessions.rows[0].path != "/wt/shared" {
		t.Errorf("row = %+v", sessions.rows)
	}
}

// Two live sessions answering to one label is the single thing `lich send`
// cannot resolve, so a name already in use is given up rather than repeated.
func TestOpenFallsBackWhenTheBranchNameIsAlreadyACard(t *testing.T) {
	svc, _, _, _, _ := closer(t)

	opened, err := svc.Open("s1", "", "", "alone", "", "")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if opened.Label != "Session 9" {
		t.Errorf("label = %q, want the project's counter — a session is already called \"alone\"", opened.Label)
	}
}
