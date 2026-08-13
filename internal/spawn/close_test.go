package spawn

import (
	"errors"
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
	sessions.projects[0].ActiveSessionID = "s2"

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

// Closing a card nobody was looking at leaves the focus where it is. The window
// applies what this writes without a choice of its own (dropClosedSession), so a
// neighbour named here would move the user off the session they are reading.
func TestCloseAnInactiveSessionLeavesTheFocusAlone(t *testing.T) {
	svc, sessions, _, _, events := closer(t)

	if _, err := svc.Close("s3", "shared-a", "", "", false); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if got := sessions.deleted[0].activeID; got != "s1" {
		t.Errorf("active session = %q, want the active card untouched", got)
	}
	closed, ok := events.events[0].data.(ClosedEvent)
	if !ok {
		t.Fatalf("event payload = %T", events.events[0].data)
	}
	if closed.ActiveID != "s1" {
		t.Errorf("the window was told %q is active, the row says s1", closed.ActiveID)
	}
}

// The row is deleted before the PTY is asked to go, so a terminal that refuses
// to close leaves nothing to undo — and a card that stayed would point at a
// session no other part of lich still knows about.
func TestCloseTakesTheCardDownEvenWhenThePTYRefuses(t *testing.T) {
	svc, sessions, _, term, events := closer(t)
	term.closeErr = errors.New("process already gone")

	if _, err := svc.Close("s1", "shared-a", "", "", false); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if len(sessions.deleted) != 1 {
		t.Fatalf("deleted = %+v, want the row gone", sessions.deleted)
	}
	if len(events.events) != 1 || events.events[0].name != ClosedEventName {
		t.Errorf("events = %+v, want the card taken down anyway", events.events)
	}
}

func TestCloseNeedsSomethingToClose(t *testing.T) {
	for _, target := range []string{"", "   "} {
		t.Run("target="+target, func(t *testing.T) {
			svc, sessions, _, _, _ := closer(t)

			_, err := svc.Close("s1", target, "", "", false)
			if err == nil {
				t.Fatal("closed a session that was never named")
			}
			if len(sessions.deleted) != 0 || len(sessions.parked) != 0 {
				t.Error("wrote something for a close with no target")
			}
		})
	}
}

// One name across two projects addresses two sessions, and closing either would
// be a guess about which agent the caller meant to take down.
func TestCloseRefusesANameThatFitsTwoSessions(t *testing.T) {
	sessions := &fakeSessions{projects: []store.Project{
		{ID: "p1", Name: "lich", Path: "/src/lich", Sessions: []store.Session{
			{ID: "s1", Label: "worker", Kind: "claude"},
		}},
		{ID: "p2", Name: "revu", Path: "/src/revu", Sessions: []store.Session{
			{ID: "s2", Label: "worker", Kind: "claude"},
		}},
	}}
	svc := New(sessions, &fakeWorktrees{}, &fakeTerminal{}, &fakeEvents{})

	_, err := svc.Close("s9", "worker", "", "", false)
	if err == nil {
		t.Fatal("closed one of two sessions that answer to the same name")
	}
	if !strings.Contains(err.Error(), "narrow it with the project") {
		t.Errorf("error = %q, want it to say how to pick one", err)
	}
	if len(sessions.deleted) != 0 {
		t.Error("deleted a session picked by a name that named two")
	}
	// Naming the project settles it.
	if _, err := svc.Close("s9", "worker", "revu", "", false); err != nil {
		t.Fatalf("Close with the project named: %v", err)
	}
	if len(sessions.deleted) != 1 || sessions.deleted[0].sessionID != "s2" {
		t.Errorf("deleted = %+v, want the session in the named project", sessions.deleted)
	}
}

func TestCloseReportsAnUnreadableWorkspace(t *testing.T) {
	svc, sessions, _, term, _ := closer(t)
	sessions.loadErr = errors.New("database is locked")

	_, err := svc.Close("s1", "shared-a", "", "", false)
	if err == nil {
		t.Fatal("closed a session out of a workspace it could not read")
	}
	if !strings.Contains(err.Error(), "database is locked") {
		t.Errorf("error = %q, want the store's own reason kept", err)
	}
	if len(term.closed) != 0 {
		t.Error("took down a PTY for a session it never resolved")
	}
}

// git's refusal is the caller's answer: the session is already gone from the
// workspace, and only git can say why the directory stayed.
func TestCloseReportsAWorktreeGitWouldNotRemove(t *testing.T) {
	svc, _, worktrees, _, _ := closer(t)
	worktrees.removeErr = errors.New("worktree is locked")

	_, err := svc.Close("s1", "alone", "", RemoveWorktree, false)
	if err == nil {
		t.Fatal("reported a checkout removed that git kept")
	}
	if !strings.Contains(err.Error(), "worktree is locked") {
		t.Errorf("error = %q, want git's own reason kept", err)
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
