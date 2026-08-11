package spawn

import (
	"strings"
	"testing"

	"github.com/omartelo/lich/internal/project"
	"github.com/omartelo/lich/internal/relay"
	"github.com/omartelo/lich/internal/store"
)

// closable is a workspace with the three shapes a close has to tell apart: a
// session in the project's own directory, two sharing one checkout, and one
// alone in another.
func closable() []store.Project {
	return []store.Project{{
		ID: "p1", Name: "lich", Path: "/src/lich", NextSeq: 9,
		ActiveSessionID: "s1",
		Sessions: []store.Session{
			{ID: "s1", Label: "Session 1", Kind: "claude"},
			{ID: "s2", Label: "shared-a", Kind: "claude", Path: "/wt/shared"},
			{ID: "s3", Label: "shared-b", Kind: "claude", Path: "/wt/shared"},
			{ID: "s4", Label: "alone", Kind: "claude", Path: "/wt/alone"},
		},
	}}
}

func closer(t *testing.T) (*Service, *fakeSessions, *fakeWorktrees, *fakeTerminal, *fakeEvents) {
	t.Helper()
	sessions := &fakeSessions{projects: closable()}
	worktrees := &fakeWorktrees{
		checkouts: []project.Worktree{
			{Name: "shared", Path: "/wt/shared"},
			{Name: "alone", Path: "/wt/alone"},
		},
		dirty: map[string]bool{},
	}
	term := &fakeTerminal{}
	events := &fakeEvents{}
	return New(sessions, worktrees, term, events), sessions, worktrees, term, events
}

func TestCloseTakesDownASessionWithNothingAtStake(t *testing.T) {
	svc, sessions, worktrees, term, events := closer(t)

	closed, err := svc.Close("s1", "shared-a", "", "", false)
	if err != nil {
		t.Fatalf("Close: %v", err)
	}
	if closed.Kept || closed.Removed {
		t.Errorf("closed = %+v, want the checkout untouched — another session is in it", closed)
	}
	if len(sessions.deleted) != 1 || sessions.deleted[0].sessionID != "s2" {
		t.Errorf("deleted = %+v", sessions.deleted)
	}
	if len(worktrees.removed) != 0 {
		t.Errorf("removed a checkout another session is working in: %+v", worktrees.removed)
	}
	if len(term.closed) == 0 {
		t.Error("left the PTY running for a session that is gone from the workspace")
	}
	if len(events.events) != 1 || events.events[0].name != ClosedEventName {
		t.Errorf("events = %+v, want the card taken down", events.events)
	}
}

// The last session in a checkout is the thing holding it, so closing it is a
// decision about that checkout — and there is nobody here to ask.
func TestCloseRefusesToDecideAWorktreesFateOnItsOwn(t *testing.T) {
	svc, sessions, _, _, _ := closer(t)

	_, err := svc.Close("s1", "alone", "", "", false)
	if err == nil {
		t.Fatal("closed the last session of a worktree without being told what to do with it")
	}
	for _, want := range []string{KeepWorktree, RemoveWorktree} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want it to name %q", err, want)
		}
	}
	if len(sessions.deleted) != 0 || len(sessions.parked) != 0 {
		t.Error("wrote something for a close that was refused")
	}
}

func TestCloseKeepingAWorktreeParksTheSession(t *testing.T) {
	svc, sessions, worktrees, _, _ := closer(t)

	closed, err := svc.Close("s1", "alone", "", KeepWorktree, false)
	if err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !closed.Kept || closed.Removed {
		t.Errorf("closed = %+v, want the checkout kept", closed)
	}
	// Parked, not deleted: the row is what carries the provider conversation
	// that opening a session on that branch again resumes.
	if len(sessions.parked) != 1 || sessions.parked[0].sessionID != "s4" {
		t.Errorf("parked = %+v, want the session kept for a resume", sessions.parked)
	}
	if len(sessions.deleted) != 0 {
		t.Errorf("deleted a session that was meant to be parked: %+v", sessions.deleted)
	}
	if len(worktrees.removed) != 0 {
		t.Errorf("removed a checkout that was meant to stay: %+v", worktrees.removed)
	}
}

func TestCloseRemovingAWorktreeTakesTheCheckoutWithIt(t *testing.T) {
	svc, sessions, worktrees, term, _ := closer(t)

	closed, err := svc.Close("s1", "alone", "", RemoveWorktree, false)
	if err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !closed.Removed || closed.Kept {
		t.Errorf("closed = %+v, want the checkout removed", closed)
	}
	if len(worktrees.removed) != 1 || worktrees.removed[0].path != "/wt/alone" {
		t.Errorf("removed = %+v", worktrees.removed)
	}
	// The parked rows go with it: one left behind would offer a resume into a
	// directory that no longer exists.
	if len(sessions.purged) != 1 || sessions.purged[0] != "/wt/alone" {
		t.Errorf("purged = %+v, want the parked rows for that checkout", sessions.purged)
	}
	if len(term.closed) == 0 || term.closed[0] != "s4" {
		t.Errorf("closed = %+v, want the PTY down before git is asked to remove its directory", term.closed)
	}
}

// What an uncommitted change loses is in no commit and on no remote, so it is
// the one thing here that needs saying twice.
func TestCloseRefusesToRemoveADirtyCheckout(t *testing.T) {
	svc, sessions, worktrees, term, _ := closer(t)
	worktrees.dirty["/wt/alone"] = true

	_, err := svc.Close("s1", "alone", "", RemoveWorktree, false)
	if err == nil {
		t.Fatal("removed a checkout with uncommitted work")
	}
	if !strings.Contains(err.Error(), "uncommitted") {
		t.Errorf("error = %q, want it to name what would be lost", err)
	}
	if len(worktrees.removed) != 0 || len(sessions.deleted) != 0 || len(term.closed) != 0 {
		t.Error("a refused removal still took something down")
	}
}

func TestCloseRemovesADirtyCheckoutWhenForced(t *testing.T) {
	svc, _, worktrees, _, _ := closer(t)
	worktrees.dirty["/wt/alone"] = true

	if _, err := svc.Close("s1", "alone", "", RemoveWorktree, true); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if len(worktrees.removed) != 1 || !worktrees.removed[0].force {
		t.Errorf("removed = %+v, want git told to force it", worktrees.removed)
	}
}

// An agent closing itself would take down the thing making the request, and the
// answer would never reach anyone.
func TestCloseRefusesTheCallersOwnSession(t *testing.T) {
	svc, sessions, _, _, _ := closer(t)

	_, err := svc.Close("s2", "shared-a", "", "", false)
	if err == nil {
		t.Fatal("a session closed itself")
	}
	if len(sessions.deleted) != 0 {
		t.Error("deleted the caller's own session")
	}
}

// One string can be a card's label and another session's roster name at the
// same time, and the two surfaces that resolve it — `lich send` and `lich
// close` — have to land on the same session. relay.resolve gives the label the
// tie; so does this.
func TestCloseResolvesALabelBeforeAnotherSessionsRosterName(t *testing.T) {
	// The roster name s5 answers to: its project's directory, then four
	// characters of its id.
	const name = "lich-s5"
	if got := relay.RosterName("/src/lich", "s5"); got != name {
		t.Fatalf("roster name = %q, want %q — the collision this test needs is gone", got, name)
	}
	sessions := &fakeSessions{projects: []store.Project{{
		ID: "p1", Name: "lich", Path: "/src/lich",
		Sessions: []store.Session{
			{ID: "s5", Label: "Session 5", Kind: "claude"},
			{ID: "s6", Label: name, Kind: "claude"},
		},
	}}}
	svc := New(sessions, &fakeWorktrees{}, &fakeTerminal{}, &fakeEvents{})

	closed, err := svc.Close("s1", name, "", "", false)
	if err != nil {
		t.Fatalf("Close: %v", err)
	}
	if closed.ID != "s6" {
		t.Errorf("closed %q, want s6 — the session %q is the label of", closed.ID, name)
	}
}

func TestCloseRefusesAnUnknownSession(t *testing.T) {
	svc, _, _, _, _ := closer(t)

	if _, err := svc.Close("s1", "ghost", "", "", false); err == nil {
		t.Fatal("closed a session that does not exist")
	}
}

func TestCloseRefusesAnUnknownWorktreeAnswer(t *testing.T) {
	svc, sessions, _, _, _ := closer(t)

	_, err := svc.Close("s1", "alone", "", "delete", false)
	if err == nil {
		t.Fatal("accepted an answer that is neither keeping nor removing")
	}
	if len(sessions.deleted) != 0 || len(sessions.parked) != 0 {
		t.Error("acted on an answer it did not understand")
	}
}

// The row that records which card is active is written by the close itself, so
// the window and the next launch land on the same session.
func TestCloseHandsTheProjectToTheNeighbour(t *testing.T) {
	svc, sessions, _, _, events := closer(t)

	if _, err := svc.Close("s1", "shared-a", "", "", false); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if got := sessions.deleted[0].activeID; got != "s3" {
		t.Errorf("active session = %q, want the card that filled the closed slot", got)
	}
	closed, ok := events.events[0].data.(ClosedEvent)
	if !ok {
		t.Fatalf("event payload = %T", events.events[0].data)
	}
	if closed.ActiveID != "s3" {
		t.Errorf("the window was told %q is active, the row says s3", closed.ActiveID)
	}
}

func TestCloseTheLastSessionOfAProjectLeavesNoneActive(t *testing.T) {
	sessions := &fakeSessions{projects: []store.Project{{
		ID: "p1", Name: "lich", Path: "/src/lich",
		Sessions: []store.Session{{ID: "only", Label: "Session 1", Kind: "claude"}},
	}}}
	svc := New(sessions, &fakeWorktrees{dirty: map[string]bool{}}, &fakeTerminal{}, nil)

	if _, err := svc.Close("", "Session 1", "", "", false); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if got := sessions.deleted[0].activeID; got != "" {
		t.Errorf("active session = %q, want none left", got)
	}
}
