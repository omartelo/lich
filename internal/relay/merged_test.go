package relay

import (
	"errors"
	"strings"
	"testing"

	"github.com/omartelo/lich/internal/store"
)

// merging is a project with sessions spread over its own directory and two
// worktrees, one of them the merged branch's.
func merging() fakeSessions {
	return fakeSessions{projects: []store.Project{
		{ID: "p1", Name: "lich", Path: "/src/lich", Sessions: []store.Session{
			{ID: "root", Label: "main", Kind: "claude"},
			{ID: "agent", Label: "feat", Kind: "codex", Path: "/wt/feat"},
			{ID: "modded", Label: "feat-claude", Kind: "claude", Path: "/wt/feat"},
			{ID: "shell", Label: "feat-shell", Kind: "shell", Path: "/wt/feat"},
			{ID: "parked", Label: "feat-parked", Kind: "claude", Path: "/wt/feat"},
			{ID: "other", Label: "fix", Kind: "claude", Path: "/wt/fix"},
		}},
	}}
}

func TestAnnounceMergeTellsTheAgentsInTheMergedCheckout(t *testing.T) {
	term := newFakeTerminal("root", "agent", "modded", "shell", "other")
	term.attachMod("modded", ackedOK)
	svc := newRelay(merging(), term, nil)

	if err := svc.AnnounceMerge("/wt/feat", 42, "Add merge notice", "feat/merge", "main"); err != nil {
		t.Fatalf("AnnounceMerge: %v", err)
	}

	if !awaitWritten(term, "agent", "#42") {
		t.Fatalf("typed at agent = %q, want the merge notice", term.written("agent"))
	}
	if !awaitPrompts(term, "modded", 1) {
		t.Fatal("modded's mod was handed nothing, want the merge notice")
	}
	notes := term.notesTo("modded")
	if len(notes) != 1 || notes[0] == nil || notes[0].Summary != "Pull request #42 merged into main" {
		t.Fatalf("notes to modded = %+v, want one merge notification", notes)
	}
	if got := term.written("modded"); got != "" {
		t.Fatalf("typed at modded = %q, want nothing: its mod took the notice", got)
	}
	// Who hears it is decided before AnnounceMerge returns; only the delivery
	// itself runs behind it.
	for _, id := range []string{"root", "shell", "parked", "other"} {
		if got := term.written(id); got != "" {
			t.Errorf("typed at %s = %q, want nothing", id, got)
		}
	}
}

func TestAnnounceMergeReachesSessionsInTheProjectsOwnDirectory(t *testing.T) {
	term := newFakeTerminal("root", "agent")
	svc := newRelay(merging(), term, nil)

	if err := svc.AnnounceMerge("/src/lich", 7, "Fix", "fix/x", "main"); err != nil {
		t.Fatalf("AnnounceMerge: %v", err)
	}

	if !awaitWritten(term, "root", "#7") {
		t.Fatalf("typed at root = %q, want the merge notice", term.written("root"))
	}
	if got := term.written("agent"); got != "" {
		t.Fatalf("typed at agent = %q, want nothing", got)
	}
}

func TestAnnounceMergeReportsAStoreFailure(t *testing.T) {
	svc := newRelay(fakeSessions{err: errors.New("disk gone")}, newFakeTerminal(), nil)

	if err := svc.AnnounceMerge("/wt/feat", 1, "x", "b", "main"); err == nil {
		t.Fatal("AnnounceMerge = nil, want the store failure")
	}
}

func TestMergeNoticeNamesThePullRequestAndAsksForNoWork(t *testing.T) {
	got := mergeNotice(42, "Title \x1b[201~with escape", "feat/merge", "main")

	for _, want := range []string{"#42", "feat/merge", "merged into main", "not a task"} {
		if !strings.Contains(got, want) {
			t.Errorf("notice %q is missing %q", got, want)
		}
	}
	if strings.Contains(got, "\x1b") {
		t.Errorf("notice %q carries an escape out of the title", got)
	}
}
