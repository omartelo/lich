package spawn

import (
	"errors"
	"strings"
	"testing"

	"github.com/omartelo/lich/internal/store"
)

// A subagent edits what the session that asked for it sees, the way Claude
// Code's own does: with no worktree named it opens in the caller's checkout,
// not in the project's main directory.
func TestOpenSubagentOpensInTheCallersWorktree(t *testing.T) {
	svc, sessions, worktrees, term, _ := newService(t)
	sessions.projects[0].Sessions[0].Path = "/wt/auth-fix"

	opened, err := svc.OpenSubagent("s1", "", "", "", "", "", false)
	if err != nil {
		t.Fatalf("OpenSubagent: %v", err)
	}
	if len(worktrees.created) != 0 {
		t.Errorf("created %+v, want the caller's checkout reused", worktrees.created)
	}
	if opened.Path != "/wt/auth-fix" || sessions.rows[0].path != "/wt/auth-fix" {
		t.Errorf("path %q / row %q, want the caller's checkout", opened.Path, sessions.rows[0].path)
	}
	if term.spawns[0].cwd != "/wt/auth-fix" {
		t.Errorf("spawned in %q, want the caller's checkout", term.spawns[0].cwd)
	}
	if term.spawns[0].setup {
		t.Error("ran the setup script in a checkout that already ran it")
	}
	// The caller already answers to the checkout's name.
	if opened.Label != "Session 4" {
		t.Errorf("label = %q, want the project's counter", opened.Label)
	}
}

func TestOpenSubagentOpensInTheCallersProjectDirectory(t *testing.T) {
	svc, sessions, _, term, _ := newService(t)

	opened, err := svc.OpenSubagent("s1", "", "", "", "", "", false)
	if err != nil {
		t.Fatalf("OpenSubagent: %v", err)
	}
	if opened.Path != "" || sessions.rows[0].path != "" || term.spawns[0].cwd != "/src/lich" {
		t.Errorf("path %q, cwd %q; want the project's own directory", opened.Path, term.spawns[0].cwd)
	}
}

func TestOpenSubagentWithAWorktreeOpensItAsOpenDoes(t *testing.T) {
	svc, sessions, worktrees, term, _ := newService(t)
	sessions.projects[0].Sessions[0].Path = "/wt/elsewhere"

	opened, err := svc.OpenSubagent("s1", "", "auth-fix", "", "", "", false)
	if err != nil {
		t.Fatalf("OpenSubagent: %v", err)
	}
	if len(worktrees.created) != 1 || opened.Path != "/wt/auth-fix" || term.spawns[0].cwd != "/wt/auth-fix" {
		t.Errorf("created %+v, path %q; want the worktree asked for", worktrees.created, opened.Path)
	}
	if opened.Label != "auth-fix" || !term.spawns[0].setup {
		t.Errorf("label %q, setup %v; want the fresh worktree's own", opened.Label, term.spawns[0].setup)
	}
}

// Workers sit under who asked for them.
func TestOpenSubagentFilesTheWorkerUnderTheCallersLabel(t *testing.T) {
	svc, sessions, _, _, events := newService(t)

	opened, err := svc.OpenSubagent("s1", "", "", "", "", "", false)
	if err != nil {
		t.Fatalf("OpenSubagent: %v", err)
	}
	if opened.Folder != "Session 3" || sessions.folders[opened.ID] != "Session 3" {
		t.Errorf("folder %q, rows %v; want the caller's label", opened.Folder, sessions.folders)
	}
	if events.events[0].data.(Session).Folder != "Session 3" {
		t.Errorf("the card was announced unfiled: %+v", events.events[0].data)
	}
}

// The caller sits in the folder with its workers, and the window hears it move.
func TestOpenSubagentFilesTheCallerBesideItsWorkers(t *testing.T) {
	svc, sessions, _, _, events := newService(t)

	if _, err := svc.OpenSubagent("s1", "", "", "", "", "", false); err != nil {
		t.Fatalf("OpenSubagent: %v", err)
	}
	if sessions.folders["s1"] != "Session 3" {
		t.Errorf("caller filed under %q, want %q", sessions.folders["s1"], "Session 3")
	}
	last := events.events[len(events.events)-1]
	filed, ok := last.data.(FiledEvent)
	if last.name != FiledEventName || !ok || filed.Folder != "Session 3" ||
		len(filed.IDs) != 1 || filed.IDs[0] != "s1" || filed.ProjectID != "p1" {
		t.Errorf("last event %+v, want the caller announced into its folder", last)
	}
}

// A caller the user already filed keeps its folder, and its workers join it:
// moving it out would undo the user's own grouping.
func TestOpenSubagentFilesTheWorkerUnderTheCallersFolder(t *testing.T) {
	svc, sessions, _, _, events := newService(t)
	sessions.projects[0].Sessions[0].Folder = "Auth"

	opened, err := svc.OpenSubagent("s1", "", "", "", "", "", false)
	if err != nil {
		t.Fatalf("OpenSubagent: %v", err)
	}
	if opened.Folder != "Auth" || sessions.folders[opened.ID] != "Auth" {
		t.Errorf("worker folder %q, rows %v; want the caller's folder", opened.Folder, sessions.folders)
	}
	if _, moved := sessions.folders["s1"]; moved {
		t.Errorf("caller was refiled: %v", sessions.folders)
	}
	for _, e := range events.events {
		if e.name == FiledEventName {
			t.Errorf("caller announced as moved: %+v", e)
		}
	}
}

func TestOpenSubagentPassesTheOverridesThrough(t *testing.T) {
	svc, sessions, _, term, _ := newService(t)

	opened, err := svc.OpenSubagent("s1", "claude", "", "", "opus", "high", true)
	if err != nil {
		t.Fatalf("OpenSubagent: %v", err)
	}
	if term.spawns[0].kind != "claude" || sessions.models[opened.ID] != "opus" ||
		sessions.efforts[opened.ID] != "high" || !sessions.ultracodes[opened.ID] {
		t.Errorf("kind %q, row %v %v %v; want every override", term.spawns[0].kind,
			sessions.models, sessions.efforts, sessions.ultracodes)
	}
}

// The mark is on the row before the terminal starts, which reads it to keep the
// worker's own subagents native.
func TestOpenSubagentMarksTheRow(t *testing.T) {
	svc, sessions, _, _, _ := newService(t)

	opened, err := svc.OpenSubagent("s1", "", "", "", "", "", false)
	if err != nil {
		t.Fatalf("OpenSubagent: %v", err)
	}
	if !sessions.subagents[opened.ID] {
		t.Errorf("subagents = %v, want the worker marked", sessions.subagents)
	}
	if plain, _ := svc.Open("s1", "", "", "", "", "", "", "", false); sessions.subagents[plain.ID] {
		t.Error("a plain Open was marked as a subagent")
	}
}

func TestOpenSubagentStartsEvenWhenTheMarkCannotBeWritten(t *testing.T) {
	svc, sessions, _, term, _ := newService(t)
	sessions.subagentErr = errors.New("disk full")

	_, err := svc.OpenSubagent("s1", "", "", "", "", "", false)
	if err == nil || !strings.Contains(err.Error(), "subagent") {
		t.Errorf("err = %v, want the mark's failure reported", err)
	}
	if len(term.spawns) != 1 {
		t.Errorf("started %d terminals, want the session running anyway", len(term.spawns))
	}
}

func TestOpenSubagentNeedsACallingSession(t *testing.T) {
	for name, fromID := range map[string]string{"no session": "", "unknown session": "gone"} {
		t.Run(name, func(t *testing.T) {
			svc, sessions, _, _, _ := newService(t)

			_, err := svc.OpenSubagent(fromID, "", "", "", "", "", false)
			if err == nil || !strings.Contains(err.Error(), "subagent") {
				t.Fatalf("err = %v, want a refusal naming the subagent", err)
			}
			if len(sessions.rows) != 0 {
				t.Errorf("wrote %d rows, want nothing opened", len(sessions.rows))
			}
		})
	}
}

// finishedWorkers is closable plus three workers: one beside its caller in a
// shared worktree, one beside a caller in the project's own directory, and one
// on a worktree of its own.
func finishedWorkers(t *testing.T) (*Service, *fakeSessions, *fakeTerminal, *fakeEvents) {
	t.Helper()
	svc, sessions, _, term, events := closer(t)
	sessions.projects[0].Sessions = append(sessions.projects[0].Sessions,
		store.Session{ID: "w1", Label: "Session 9", Kind: "claude", Path: "/wt/shared", OriginSessionID: "s2"},
		store.Session{ID: "w2", Label: "Session 10", Kind: "claude", OriginSessionID: "s1"},
		store.Session{ID: "w3", Label: "subagent/docs-ab12", Kind: "claude", Path: "/wt/docs", OriginSessionID: "s2"},
	)
	return svc, sessions, term, events
}

// A worker in its caller's checkout has nothing of its own to keep once it
// reported, so it closes the way a native subagent ends: parked, card down.
func TestCloseFinishedWorkerClosesOneSharingItsCallersCheckout(t *testing.T) {
	for _, id := range []string{"w1", "w2"} {
		svc, sessions, term, events := finishedWorkers(t)

		if err := svc.CloseFinishedWorker(id); err != nil {
			t.Fatalf("CloseFinishedWorker(%s): %v", id, err)
		}
		if len(sessions.parked) != 1 || sessions.parked[0].sessionID != id {
			t.Errorf("%s: parked = %+v, want the worker parked", id, sessions.parked)
		}
		if len(term.closed) != 1 || term.closed[0] != id {
			t.Errorf("%s: closed PTYs = %v, want the worker's", id, term.closed)
		}
		if len(events.events) != 1 || events.events[0].name != ClosedEventName {
			t.Errorf("%s: events = %+v, want its card taken down", id, events.events)
		}
	}
}

// A worker on a worktree of its own holds work nobody else has, so it stays
// open for the user, as does one whose caller is gone (nothing says whose
// checkout it shared) and one the user pinned.
func TestCloseFinishedWorkerKeepsOneWithSomethingToKeep(t *testing.T) {
	cases := map[string]func(*fakeSessions){
		"own worktree": func(*fakeSessions) {},
		"caller gone": func(s *fakeSessions) {
			s.projects[0].Sessions[1].ID = "elsewhere"
		},
		"pinned": func(s *fakeSessions) {
			last := len(s.projects[0].Sessions) - 1
			s.projects[0].Sessions[last].Path = "/wt/shared"
			s.projects[0].Sessions[last].Pinned = true
		},
	}
	for name, arrange := range cases {
		svc, sessions, term, events := finishedWorkers(t)
		arrange(sessions)
		id := "w3"
		if name == "caller gone" {
			id = "w1"
		}

		if err := svc.CloseFinishedWorker(id); err != nil {
			t.Fatalf("%s: CloseFinishedWorker: %v", name, err)
		}
		if len(sessions.parked) != 0 || len(term.closed) != 0 || len(events.events) != 0 {
			t.Errorf("%s: parked %+v, closed %v, events %+v; want the worker left open",
				name, sessions.parked, term.closed, events.events)
		}
	}
}

// A worker the user already closed is not an error: there is nothing to do.
func TestCloseFinishedWorkerIgnoresAWorkerAlreadyGone(t *testing.T) {
	svc, sessions, term, _ := finishedWorkers(t)

	if err := svc.CloseFinishedWorker("nobody"); err != nil {
		t.Fatalf("CloseFinishedWorker: %v", err)
	}
	if len(sessions.parked) != 0 || len(term.closed) != 0 {
		t.Errorf("parked %+v, closed %v; want nothing touched", sessions.parked, term.closed)
	}
}
