package spawn

import (
	"os"
	"strings"
	"testing"

	"github.com/omartelo/lich/internal/store"
)

// parked runs the close against the real store rather than the fake, because
// what these tests are about is not which method was called but what survives
// the call: a row the workspace stops listing and the history starts to. Only
// SQLite can answer that.
//
// The workspace it builds is the one a close has to tell apart: a session in the
// project's own directory, two sharing one checkout, and one alone in another.
func parked(t *testing.T) (*Service, *store.Service, *fakeWorktrees) {
	t.Helper()
	rows := realStore(t)
	if err := rows.AddProject("p1", "lich", "/src/lich"); err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	for _, s := range []struct{ id, label, path string }{
		{"s1", "Session 1", ""},
		{"s2", "shared-a", "/wt/shared"},
		{"s3", "shared-b", "/wt/shared"},
		{"s4", "alone", "/wt/alone"},
	} {
		if err := rows.AddSession("p1", s.id, s.label, "claude", s.path, 9, ""); err != nil {
			t.Fatalf("AddSession %q: %v", s.id, err)
		}
	}
	worktrees := &fakeWorktrees{dirty: map[string]bool{}}
	return New(rows, worktrees, &fakeTerminal{}, nil), rows, worktrees
}

// realStore opens a store on disk with the user's config directory redirected
// into a temporary one.
//
// XDG_CONFIG_HOME alone is a Linux-only redirect: os.UserConfigDir reads
// $HOME/Library/Application Support on macOS and %AppData% on Windows, so a test
// setting only XDG would write its database into the real user's config
// directory on the other two — agreeing with every assertion here and leaving a
// file behind on a machine it was never supposed to touch. All three are set,
// and the answer is read back from the standard library rather than rebuilt,
// because the per-OS layout is its rule and not lich's.
func realStore(t *testing.T) *store.Service {
	t.Helper()
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	t.Setenv("AppData", root)
	t.Setenv("HOME", root)
	dir, err := os.UserConfigDir()
	if err != nil {
		t.Fatalf("resolve config directory: %v", err)
	}
	if !strings.HasPrefix(dir, root) {
		t.Fatalf("config dir %q is outside the test's temp dir %q", dir, root)
	}
	rows, err := store.New()
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = rows.Close() })
	return rows
}

// listed reports whether the workspace still shows the session, and whether the
// history does. A parked row is in the second and not the first; a deleted one
// is in neither.
func listed(t *testing.T, rows *store.Service, id string) (open, history bool) {
	t.Helper()
	projects, err := rows.LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	for _, p := range projects {
		for _, sess := range p.Sessions {
			if sess.ID == id {
				open = true
			}
		}
	}
	closed, err := rows.ClosedSessions()
	if err != nil {
		t.Fatalf("ClosedSessions: %v", err)
	}
	for _, sess := range closed {
		if sess.ID == id {
			history = true
		}
	}
	return open, history
}

// A session sharing its checkout closes without deciding anything about that
// checkout — which is not a reason to destroy the row. `lich close` and the MCP
// close_session tool go through here, and a deleted row takes the conversation,
// the cost ledgers and the name the user chose with it.
func TestClosingASharedCheckoutParksTheSession(t *testing.T) {
	svc, rows, worktrees := parked(t)

	if _, err := svc.Close("s1", "shared-a", "", "", false); err != nil {
		t.Fatalf("Close: %v", err)
	}
	open, history := listed(t, rows, "s2")
	if open {
		t.Error("the workspace still lists a session that was closed")
	}
	if !history {
		t.Error("the closed session is in no history — its row was destroyed, not parked")
	}
	if len(worktrees.removed) != 0 {
		t.Errorf("removed = %+v, want the checkout another session is in left alone", worktrees.removed)
	}
	// The session it shared the checkout with is untouched.
	if open, _ := listed(t, rows, "s3"); !open {
		t.Error("closing one session took its neighbour in the same checkout with it")
	}
}

// A session in the project's own directory has no checkout at all, so there is
// nothing to remove and nothing to decide — and still a conversation to keep.
func TestClosingASessionInTheProjectDirectoryParksIt(t *testing.T) {
	svc, rows, _ := parked(t)

	if _, err := svc.Close("s2", "Session 1", "", "", false); err != nil {
		t.Fatalf("Close: %v", err)
	}
	open, history := listed(t, rows, "s1")
	if open {
		t.Error("the workspace still lists a session that was closed")
	}
	if !history {
		t.Error("the closed session is in no history — its row was destroyed, not parked")
	}
}

// The pin the other way: a row must not outlive the directory it points at, so
// the one close that removes a checkout still deletes for good. It is the only
// one that does.
func TestClosingTheLastSessionAndRemovingItsWorktreeDeletesTheRow(t *testing.T) {
	svc, rows, worktrees := parked(t)

	if _, err := svc.Close("s1", "alone", "", "remove", false); err != nil {
		t.Fatalf("Close: %v", err)
	}
	open, history := listed(t, rows, "s4")
	if open || history {
		t.Errorf(
			"session still listed (open %v, history %v) — a resume would open a directory that is gone",
			open, history,
		)
	}
	if len(worktrees.removed) != 1 || worktrees.removed[0].path != "/wt/alone" {
		t.Errorf("removed = %+v, want the checkout deleted", worktrees.removed)
	}
}

// Keeping the checkout parks too, and the history is where the two parked closes
// meet: whichever way a session was closed, opening it again starts from a row
// that is still there.
func TestAParkedSessionIsFoundInTheHistoryWithItsName(t *testing.T) {
	svc, rows, _ := parked(t)

	if _, err := svc.Close("s1", "shared-a", "", "", false); err != nil {
		t.Fatalf("Close shared-a: %v", err)
	}
	if _, err := svc.Close("s1", "alone", "", "keep", false); err != nil {
		t.Fatalf("Close alone: %v", err)
	}

	closed, err := rows.ClosedSessions()
	if err != nil {
		t.Fatalf("ClosedSessions: %v", err)
	}
	found := map[string]store.ClosedSession{}
	for _, sess := range closed {
		found[sess.ID] = sess
	}
	if len(found) != 2 {
		t.Fatalf("history = %+v, want both closed sessions", closed)
	}
	if got := found["s2"]; got.Label != "shared-a" || got.Path != "/wt/shared" || got.ProjectName != "lich" {
		t.Errorf("history row = %+v, want the session identifiable by what the user called it", got)
	}
	if got := found["s4"]; got.Label != "alone" || got.Path != "/wt/alone" {
		t.Errorf("history row = %+v, want the kept checkout's session findable too", got)
	}
}
