package spawn

import (
	"errors"
	"strings"
	"testing"
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
