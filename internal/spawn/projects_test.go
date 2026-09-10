package spawn

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/omartelo/lich/internal/project"
	"github.com/omartelo/lich/internal/store"
)

// emittedNames is the order the window is told things in, which is the whole
// contract between a project event and the session event behind it.
func emittedNames(events *fakeEvents) []string {
	names := make([]string, 0, len(events.events))
	for _, e := range events.events {
		names = append(names, e.name)
	}
	return names
}

// TestOpenAdoptsADirectoryNotOnScreen proves the one thing a path can do that a
// name cannot: put a project the window is not holding on screen, and open the
// session in it.
func TestOpenAdoptsADirectoryNotOnScreen(t *testing.T) {
	dir := t.TempDir()
	svc, sessions, _, term, events := newService(t)

	opened, err := svc.Open("s1", dir, "", "", "", "")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	want, err := project.Identify(dir)
	if err != nil {
		t.Fatalf("Identify: %v", err)
	}
	if opened.ProjectID != want.ID || opened.Project != want.Name {
		t.Errorf("opened in %q (%q), want %q (%q)", opened.Project, opened.ProjectID, want.Name, want.ID)
	}
	// A brand-new project starts its own counter rather than continuing the
	// caller's.
	if opened.Label != "Session 1" || opened.NextSeq != 2 {
		t.Errorf("label %q, next %d; want Session 1 and 2", opened.Label, opened.NextSeq)
	}
	if len(term.spawns) != 1 || term.spawns[0].cwd != want.Path {
		t.Errorf("started %+v, want one PTY in %q", term.spawns, want.Path)
	}
	if len(sessions.rows) != 1 || sessions.rows[0].projectID != want.ID {
		t.Errorf("wrote %+v, want one row in %q", sessions.rows, want.ID)
	}
	// The tab has to land before the card: a session-opened whose project is
	// not on screen yet is dropped by the window (adoptSession).
	if names := emittedNames(events); len(names) != 2 ||
		names[0] != ProjectOpenedEventName || names[1] != OpenedEventName {
		t.Fatalf("emitted %v, want the project before the session", names)
	}
	announced, ok := events.events[0].data.(store.Project)
	if !ok || announced.ID != want.ID || announced.Path != want.Path {
		t.Errorf("announced %+v, want the whole project row", events.events[0].data)
	}
}

// TestOpenReopensAProjectFromTheHistory proves a closed project is reopened
// rather than duplicated — with the sessions it was closed with, and under the
// id it was closed under. The fixture's id is not the one this path hashes to,
// which is what a relocated project looks like: the row keeps the id its
// sessions and its worktree directory hang off, and deriving a fresh one from
// the path would open a second project on the same directory.
func TestOpenReopensAProjectFromTheHistory(t *testing.T) {
	dir := t.TempDir()
	svc, sessions, _, _, events := newService(t)
	sessions.closed = []store.Project{{
		ID: "relocated", Name: "revu", Path: dir, NextSeq: 7,
		Sessions: []store.Session{{ID: "parked", Label: "Session 6", Kind: "claude"}},
	}}

	opened, err := svc.Open("s1", dir, "", "", "", "")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	if opened.ProjectID != "relocated" || opened.Project != "revu" {
		t.Errorf("opened in %q (%q), want the stored row", opened.Project, opened.ProjectID)
	}
	// The label continues the counter the project was closed with; a session
	// that repeats a live label is one `lich send` cannot address.
	if opened.Label != "Session 7" || opened.NextSeq != 8 {
		t.Errorf("label %q, next %d; want Session 7 and 8", opened.Label, opened.NextSeq)
	}
	announced, ok := events.events[0].data.(store.Project)
	if !ok || len(announced.Sessions) != 1 || announced.Sessions[0].ID != "parked" {
		t.Errorf("announced %+v, want the parked session with it", events.events[0].data)
	}
}

// TestOpenFindsAnOpenProjectByPath proves a path that names a project already on
// screen opens a session in it and nothing else: no second row, no tab event.
func TestOpenFindsAnOpenProjectByPath(t *testing.T) {
	dir := t.TempDir()
	svc, sessions, _, _, events := newService(t)
	sessions.projects[1].Path = dir

	opened, err := svc.Open("s1", dir+string(filepath.Separator), "", "", "", "")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	if opened.ProjectID != "p2" {
		t.Errorf("opened in %q, want the project already holding that directory", opened.ProjectID)
	}
	if names := emittedNames(events); len(names) != 1 || names[0] != OpenedEventName {
		t.Errorf("emitted %v, want only the session", names)
	}
}

// TestOpenRefusesAPathItCannotTrust proves the boundary answers before anything
// is written. A relative path is the one that matters: lich would resolve it
// against its own window's directory rather than the caller's, so a project
// would open — just not the one asked for.
func TestOpenRefusesAPathItCannotTrust(t *testing.T) {
	svc, sessions, _, term, events := newService(t)

	for _, tc := range []struct{ name, path, want string }{
		{"relative", "./repo", "relative path"},
		{"missing", filepath.Join(t.TempDir(), "nope"), "no directory at"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := svc.Open("s1", tc.path, "", "", "", ""); err == nil ||
				!strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Open(%q) = %v, want it to name %q", tc.path, err, tc.want)
			}
		})
	}
	if len(sessions.rows) != 0 || len(term.spawns) != 0 || len(events.events) != 0 {
		t.Errorf("a refused path still wrote %+v / %+v / %+v", sessions.rows, term.spawns, events.events)
	}
}

// TestNarrowingTakesAPathToo proves --project reads the same wherever it only
// narrows a search: the session lives in the project at that directory.
func TestNarrowingTakesAPathToo(t *testing.T) {
	dir := t.TempDir()
	svc, sessions, _, _, _ := newService(t)
	sessions.projects[1].Path = dir
	sessions.projects[1].Sessions = []store.Session{{ID: "s2", Label: "Session 3"}}

	// "Session 3" is a label both projects hold, which is what --project is for.
	renamed, err := svc.Rename("s1", "Session 3", dir, "review")
	if err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if renamed.ID != "s2" {
		t.Errorf("renamed %q, want the session in the project at that path", renamed.ID)
	}
}

// TestWorktreesTakeAPathToo proves --project reads the same on every command
// that has one: a path reaches a project on screen, and only an open is allowed
// to put one there.
func TestWorktreesTakeAPathToo(t *testing.T) {
	dir := t.TempDir()
	svc, sessions, worktrees, _, _ := newService(t)
	sessions.projects[1].Path = dir
	worktrees.checkouts = []project.Worktree{{Name: "feature", Path: "/wt/feature"}}

	found, err := svc.Worktrees("s1", dir)
	if err != nil {
		t.Fatalf("Worktrees: %v", err)
	}
	if len(found) != 1 || found[0].Name != "feature" {
		t.Errorf("Worktrees = %+v, want the checkout of the project at that path", found)
	}

	elsewhere := t.TempDir()
	if _, err := svc.Worktrees("s1", elsewhere); err == nil ||
		!strings.Contains(err.Error(), "no open project at") {
		t.Errorf("Worktrees(%q) = %v, want it to say no project is open there", elsewhere, err)
	}
	if len(sessions.projects) != 2 {
		t.Errorf("listing worktrees opened a project: %+v", sessions.projects)
	}
}
