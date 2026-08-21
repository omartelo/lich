package spawn

import (
	"errors"
	"strings"
	"testing"

	"github.com/omartelo/lich/internal/store"
)

func TestRenameGivesTheCardTheNameAndAnnouncesIt(t *testing.T) {
	svc, sessions, _, _, events := closer(t)

	renamed, err := svc.Rename("s1", "alone", "", "the login bug")
	if err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if renamed.Previous != "alone" || renamed.Label != "the login bug" {
		t.Errorf("renamed = %+v, want both ends of the change", renamed)
	}
	if sessions.renamed["s4"] != "the login bug" {
		t.Errorf("renamed = %v, want the target session written under the new name", sessions.renamed)
	}
	if len(events.events) != 1 || events.events[0].name != RenamedEventName {
		t.Errorf("events = %+v, want the card told to relabel", events.events)
	}
}

// The caller's own session is the one an agent can reach without discovery —
// list_sessions shows it every session but itself.
func TestRenameWithoutATargetRenamesTheCaller(t *testing.T) {
	svc, sessions, _, _, _ := closer(t)

	renamed, err := svc.Rename("s2", "", "", "shared-worker")
	if err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if renamed.ID != "s2" || sessions.renamed["s2"] != "shared-worker" {
		t.Errorf("renamed %+v / %v, want the calling session", renamed, sessions.renamed)
	}
}

// Two sessions under one label is the one thing `lich send` cannot resolve, so
// the write that would create the pair is refused rather than made.
func TestRenameRefusesALabelAnotherSessionHolds(t *testing.T) {
	svc, sessions, _, _, events := closer(t)

	_, err := svc.Rename("s1", "alone", "", "Shared-A")
	if err == nil {
		t.Fatal("renamed a session onto a label another one already answers to")
	}
	if !strings.Contains(err.Error(), "Shared-A") {
		t.Errorf("error = %q, want it to name the label that is taken", err)
	}
	if len(sessions.renamed) != 0 || len(events.events) != 0 {
		t.Error("wrote something for a rename that was refused")
	}
}

// Renaming a session to what it already answers to is not competition with
// itself — only a differently-cased spelling of its own name.
func TestRenameAcceptsTheSessionsOwnLabelBack(t *testing.T) {
	svc, sessions, _, _, _ := closer(t)

	if _, err := svc.Rename("s1", "alone", "", "Alone"); err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if sessions.renamed["s4"] != "Alone" {
		t.Errorf("renamed = %v, want the recased name written", sessions.renamed)
	}
}

func TestRenameRefusesAnEmptyName(t *testing.T) {
	svc, sessions, _, _, _ := closer(t)

	if _, err := svc.Rename("s1", "alone", "", "   "); err == nil {
		t.Fatal("accepted a name that is nothing but whitespace")
	}
	if len(sessions.renamed) != 0 {
		t.Error("wrote an empty name")
	}
}

// A command line outside any session that names no target has nothing to
// rename, and is told that rather than handed the resolver's "no session named".
func TestRenameOutsideASessionNeedsATarget(t *testing.T) {
	svc, _, _, _, _ := closer(t)

	_, err := svc.Rename("", "", "", "planner")
	if err == nil {
		t.Fatal("renamed something for a caller that is in no session")
	}
	if !strings.Contains(err.Error(), "name the session") {
		t.Errorf("error = %q, want it to say what the caller must do", err)
	}
}

// A failed write leaves the card alone: the event says the name changed, and a
// window told that over a row that still holds the old name shows a name that
// the next reload takes away.
func TestRenameThatCannotBeWrittenAnnouncesNothing(t *testing.T) {
	svc, sessions, _, _, events := closer(t)
	sessions.renameErr = errors.New("disk is gone")

	if _, err := svc.Rename("s1", "alone", "", "planner"); err == nil {
		t.Fatal("reported a rename the store refused")
	}
	if len(events.events) != 0 {
		t.Errorf("events = %+v, want none for a rename that was not written", events.events)
	}
}

// The same label in two projects is ambiguous, exactly as it is for a close.
func TestRenameNarrowsAnAmbiguousLabelWithTheProject(t *testing.T) {
	svc, sessions, _, _, _ := closer(t)
	twin := sessions.projects[0]
	twin.ID, twin.Name, twin.Path = "p2", "other", "/src/other"
	twin.Sessions = []store.Session{{ID: "s9", Label: "alone", Kind: "claude", Path: "/wt/alone"}}
	sessions.projects = append(sessions.projects, twin)

	if _, err := svc.Rename("s1", "alone", "", "planner"); err == nil {
		t.Fatal("picked one of two sessions answering to the same label")
	}
	renamed, err := svc.Rename("s1", "alone", "other", "planner")
	if err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if renamed.ID != "s9" {
		t.Errorf("renamed %q, want the session in the project that was named", renamed.ID)
	}
}
