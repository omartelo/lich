package spawn

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/omartelo/lich/internal/store"
)

// runWorkspace is one project rooted at a real directory, so project.RunScript
// has a checkout to read the run script out of.
func runWorkspace(t *testing.T, script string) (*Service, *fakeSessions, *fakeTerminal, *fakeEvents, string) {
	t.Helper()
	root := t.TempDir()
	if script != "" {
		if err := os.MkdirAll(filepath.Join(root, ".lich"), 0o755); err != nil {
			t.Fatalf("mkdir .lich: %v", err)
		}
		if err := os.WriteFile(
			filepath.Join(root, ".lich", "run-worktree.sh"), []byte(script), 0o755,
		); err != nil {
			t.Fatalf("write run script: %v", err)
		}
	}
	sessions := &fakeSessions{projects: []store.Project{
		{ID: "p1", Name: "lich", Path: root, NextSeq: 4},
	}}
	term := &fakeTerminal{}
	events := &fakeEvents{}
	return New(sessions, &fakeWorktrees{branch: "main"}, term, events), sessions, term, events, root
}

func TestRunOpensATerminalOnTheScript(t *testing.T) {
	svc, sessions, term, events, _ := runWorkspace(t, "pnpm dev --port $LICH_WORKTREE_PORT\n")

	opened, err := svc.Run("p1", "/wt/feature")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if opened.Kind != "shell" {
		t.Errorf("kind = %q, want shell", opened.Kind)
	}
	if opened.Label != "pnpm dev --port $LICH_WORKTREE_PORT" {
		t.Errorf("label = %q, want the command", opened.Label)
	}
	if opened.Path != "/wt/feature" {
		t.Errorf("path = %q, want the checkout", opened.Path)
	}
	if opened.NextSeq != 5 {
		t.Errorf("nextSeq = %d, want 5", opened.NextSeq)
	}
	if got := sessions.entrypoints[opened.ID]; got != "pnpm dev --port $LICH_WORKTREE_PORT" {
		t.Errorf("entrypoint = %q, want the script", got)
	}
	// The PTY is started here rather than left to the window: that is the whole
	// point — a run card nobody clicks still runs the app.
	if len(term.spawns) != 1 {
		t.Fatalf("spawns = %d, want 1", len(term.spawns))
	}
	spawn := term.spawns[0]
	if spawn.cwd != "/wt/feature" || spawn.kind != "shell" || spawn.setup {
		t.Errorf("spawn = %+v, want the checkout on a shell with no setup", spawn)
	}
	if len(events.events) != 1 || events.events[0].name != OpenedEventName {
		t.Errorf("events = %+v, want one session-opened", events.events)
	}
}

func TestRunInTheProjectsOwnDirectoryStoresNoPath(t *testing.T) {
	svc, _, term, _, root := runWorkspace(t, "task dev")

	opened, err := svc.Run("p1", root)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// The store's spelling for "the project's own directory" is the empty
	// string; the PTY still starts in the real one.
	if opened.Path != "" {
		t.Errorf("path = %q, want empty for the main checkout", opened.Path)
	}
	if term.spawns[0].cwd != root {
		t.Errorf("cwd = %q, want %q", term.spawns[0].cwd, root)
	}
}

func TestRunNamesTheCardAfterTheFirstLine(t *testing.T) {
	svc, _, _, _, _ := runWorkspace(t, "export FOO=1\npnpm dev\n")

	opened, err := svc.Run("p1", "/wt/feature")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if opened.Label != "export FOO=1" {
		t.Errorf("label = %q, want the script's first line", opened.Label)
	}
}

func TestRunWithoutAScriptSaysSo(t *testing.T) {
	svc, sessions, term, _, _ := runWorkspace(t, "")

	_, err := svc.Run("p1", "/wt/feature")
	if err == nil {
		t.Fatal("Run: want an error when the project ships no run script")
	}
	if !strings.Contains(err.Error(), "run-worktree.sh") {
		t.Errorf("error = %q, want it to name the file", err)
	}
	// Nothing was created: a project with no command has no card to show.
	if len(sessions.rows) != 0 || len(term.spawns) != 0 {
		t.Errorf("rows = %d, spawns = %d, want none", len(sessions.rows), len(term.spawns))
	}
}

func TestRunRefusesAnUnknownProject(t *testing.T) {
	svc, _, _, _, _ := runWorkspace(t, "pnpm dev")

	if _, err := svc.Run("nope", "/wt/feature"); err == nil {
		t.Fatal("Run: want an error for a project that is not open")
	}
	if _, err := svc.Run("", "/wt/feature"); err == nil {
		t.Fatal("Run: want an error with no project")
	}
	if _, err := svc.Run("p1", ""); err == nil {
		t.Fatal("Run: want an error with no checkout")
	}
}

// A card whose entrypoint could not be recorded would open a bare shell, which
// is not what the user asked for — so the write failing fails the call.
func TestRunFailsWhenTheEntrypointCannotBeRecorded(t *testing.T) {
	svc, sessions, term, _, _ := runWorkspace(t, "pnpm dev")
	sessions.entrypointErr = errors.New("disk full")

	if _, err := svc.Run("p1", "/wt/feature"); err == nil {
		t.Fatal("Run: want the entrypoint write to fail the call")
	}
	if len(term.spawns) != 0 {
		t.Errorf("spawns = %d, want none — the shell would not run the app", len(term.spawns))
	}
}
