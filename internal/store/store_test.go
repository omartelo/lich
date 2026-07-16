package store

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

// newTestStore opens a throwaway database under the test's temp directory.
func newTestStore(t *testing.T) *Service {
	t.Helper()
	svc, err := open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open test store: %v", err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	return svc
}

func TestLoadStateRestoresOpenProjectsAndSessions(t *testing.T) {
	svc := newTestStore(t)

	if err := svc.AddProject("p1", "alpha", "/tmp/alpha"); err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	if err := svc.AddSession("p1", "s1", "Session 1", "", "", 2); err != nil {
		t.Fatalf("AddSession: %v", err)
	}
	if err := svc.AddSession("p1", "s2", "Session 2", "", "", 3); err != nil {
		t.Fatalf("AddSession: %v", err)
	}

	projects, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("got %d projects, want 1", len(projects))
	}
	p := projects[0]
	if p.ID != "p1" || p.Name != "alpha" || p.Path != "/tmp/alpha" {
		t.Errorf("project metadata = %+v", p)
	}
	if p.NextSeq != 3 {
		t.Errorf("NextSeq = %d, want 3", p.NextSeq)
	}
	if p.ActiveSessionID != "s2" {
		t.Errorf("ActiveSessionID = %q, want s2", p.ActiveSessionID)
	}
	if len(p.Sessions) != 2 || p.Sessions[0].ID != "s1" || p.Sessions[1].Label != "Session 2" {
		t.Errorf("sessions = %+v", p.Sessions)
	}
}

// TestSessionPathPersistsAndDefaults proves a worktree session's path survives
// a reload and that rows written before the column existed load as "".
func TestSessionPathPersistsAndDefaults(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	if err := svc.AddSession("p1", "s1", "mellow-otter", "claude", "/data/wt/mellow-otter", 2); err != nil {
		t.Fatalf("AddSession: %v", err)
	}
	// Row without the path column, as written before the migration.
	if _, err := svc.db.Exec(
		`INSERT INTO sessions (id, project_id, label, kind) VALUES ('s2', 'p1', 'Session 2', 'claude')`,
	); err != nil {
		t.Fatalf("insert legacy row: %v", err)
	}

	projects, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	sessions := projects[0].Sessions
	if sessions[0].Path != "/data/wt/mellow-otter" || sessions[1].Path != "" {
		t.Errorf("paths = %q, %q; want /data/wt/mellow-otter, empty", sessions[0].Path, sessions[1].Path)
	}
}

// TestSessionKindPersistsAndDefaults proves kind survives a reload and that an
// empty kind falls back to "claude" (rows written before the column existed).
func TestSessionKindPersistsAndDefaults(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	if err := svc.AddSession("p1", "s1", "Session 1", "shell", "", 2); err != nil {
		t.Fatalf("AddSession: %v", err)
	}
	if err := svc.AddSession("p1", "s2", "Session 2", "", "", 3); err != nil {
		t.Fatalf("AddSession: %v", err)
	}

	projects, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	sessions := projects[0].Sessions
	if sessions[0].Kind != "shell" || sessions[1].Kind != "claude" {
		t.Errorf("kinds = %q, %q; want shell, claude", sessions[0].Kind, sessions[1].Kind)
	}
}

// TestSetClaudeSessionPersistsAndDefaults proves the Claude session id survives
// a reload, defaults to "" before the SessionStart hook reports it, and that a
// re-report overwrites with the latest id.
func TestSetClaudeSessionPersistsAndDefaults(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddSession("p1", "s1", "Session 1", "", "", 2)
	_ = svc.AddSession("p1", "s2", "Session 2", "", "", 3)

	if err := svc.SetClaudeSession("s1", "uuid-abc"); err != nil {
		t.Fatalf("SetClaudeSession: %v", err)
	}
	// Re-report (e.g. after a resume) overwrites with the newest id.
	if err := svc.SetClaudeSession("s1", "uuid-def"); err != nil {
		t.Fatalf("SetClaudeSession re-report: %v", err)
	}

	sessions := mustLoadSessions(t, svc)
	if sessions[0].ClaudeSessionID != "uuid-def" {
		t.Errorf("s1 claude id = %q, want uuid-def", sessions[0].ClaudeSessionID)
	}
	if sessions[1].ClaudeSessionID != "" {
		t.Errorf("s2 claude id = %q, want empty", sessions[1].ClaudeSessionID)
	}
}

// TestSetClaudeSessionUnknownSessionNoop proves reporting for a session whose
// row does not exist (the hook racing persistence) is not an error.
func TestSetClaudeSessionUnknownSessionNoop(t *testing.T) {
	svc := newTestStore(t)
	if err := svc.SetClaudeSession("ghost", "uuid-x"); err != nil {
		t.Errorf("SetClaudeSession unknown = %v, want nil", err)
	}
}

// mustLoadSessions loads the single test project's sessions or fails.
func mustLoadSessions(t *testing.T, svc *Service) []Session {
	t.Helper()
	projects, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("got %d projects, want 1", len(projects))
	}
	return projects[0].Sessions
}

func TestCloseProjectHidesButKeepsSessions(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddSession("p1", "s1", "Session 1", "", "", 2)

	if err := svc.CloseProject("p1"); err != nil {
		t.Fatalf("CloseProject: %v", err)
	}

	projects, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if len(projects) != 0 {
		t.Fatalf("closed project still loaded: %+v", projects)
	}

	// Reopening restores the stored session, proving close did not cascade.
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	projects, _ = svc.LoadState()
	if len(projects) != 1 || len(projects[0].Sessions) != 1 {
		t.Fatalf("reopened project lost its sessions: %+v", projects)
	}
}

func TestDeleteSessionRemovesRowAndUpdatesActive(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddSession("p1", "s1", "Session 1", "", "", 2)
	_ = svc.AddSession("p1", "s2", "Session 2", "", "", 3)

	if err := svc.DeleteSession("p1", "s2", "s1"); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}

	projects, _ := svc.LoadState()
	p := projects[0]
	if len(p.Sessions) != 1 || p.Sessions[0].ID != "s1" {
		t.Errorf("sessions after delete = %+v", p.Sessions)
	}
	if p.ActiveSessionID != "s1" {
		t.Errorf("ActiveSessionID = %q, want s1", p.ActiveSessionID)
	}
}

func TestRenameAndActivateSession(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddSession("p1", "s1", "Session 1", "", "", 2)
	_ = svc.AddSession("p1", "s2", "Session 2", "", "", 3)

	if err := svc.RenameSession("s1", "build"); err != nil {
		t.Fatalf("RenameSession: %v", err)
	}
	if err := svc.SetActiveSession("p1", "s1"); err != nil {
		t.Fatalf("SetActiveSession: %v", err)
	}

	projects, _ := svc.LoadState()
	p := projects[0]
	if p.Sessions[0].Label != "build" {
		t.Errorf("label = %q, want build", p.Sessions[0].Label)
	}
	if p.ActiveSessionID != "s1" {
		t.Errorf("ActiveSessionID = %q, want s1", p.ActiveSessionID)
	}
}

// TestSetSessionTitleRespectsManualRename proves the ai-title only sets the
// label while it is still automatic: it applies to a fresh session, reports
// that it changed, and no-ops (reporting false) once the user has renamed.
func TestSetSessionTitleRespectsManualRename(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddSession("p1", "s1", "Session 1", "", "", 2)

	applied, err := svc.SetSessionTitle("s1", "Fixing the auth bug")
	if err != nil {
		t.Fatalf("SetSessionTitle: %v", err)
	}
	if !applied {
		t.Fatal("SetSessionTitle on an auto label = false, want true")
	}
	if got := mustLoadSessions(t, svc)[0].Label; got != "Fixing the auth bug" {
		t.Errorf("label = %q, want the title", got)
	}

	// A user rename clears the auto flag; the next title must not stomp it.
	if err := svc.RenameSession("s1", "my build"); err != nil {
		t.Fatalf("RenameSession: %v", err)
	}
	applied, err = svc.SetSessionTitle("s1", "A different title")
	if err != nil {
		t.Fatalf("SetSessionTitle after rename: %v", err)
	}
	if applied {
		t.Fatal("SetSessionTitle after a manual rename = true, want false")
	}
	if got := mustLoadSessions(t, svc)[0].Label; got != "my build" {
		t.Errorf("manual label was stomped: %q", got)
	}
}

func TestDatabasePath(t *testing.T) {
	t.Setenv("LICH_DEV", "")
	path, err := databasePath()
	if err != nil {
		t.Fatalf("databasePath: %v", err)
	}
	if filepath.Base(path) != "lich.db" || filepath.Base(filepath.Dir(path)) != "lich" {
		t.Errorf("path = %q, want .../lich/lich.db", path)
	}
}

func TestDatabasePathDev(t *testing.T) {
	t.Setenv("LICH_DEV", "1")
	path, err := databasePath()
	if err != nil {
		t.Fatalf("databasePath: %v", err)
	}
	if filepath.Base(path) != "lich-dev.db" {
		t.Errorf("path = %q, want .../lich/lich-dev.db", path)
	}
}

func TestNewCreatesDatabaseUnderConfigDir(t *testing.T) {
	tmp := t.TempDir()
	// Redirect the OS config dir to tmp: XDG_CONFIG_HOME wins on Linux, HOME on
	// macOS. Either way New writes under the test's temp directory, not the real
	// user config.
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("HOME", tmp)

	svc, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer svc.Close()

	want, err := databasePath()
	if err != nil {
		t.Fatalf("databasePath: %v", err)
	}
	if _, err := os.Stat(want); err != nil {
		t.Errorf("database not created at %q: %v", want, err)
	}
}

func TestOpenFailsWhenParentIsAFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	// The db's parent directory cannot be created under an existing file.
	if _, err := open(filepath.Join(file, "lich.db")); err == nil {
		t.Error("open under a file parent = nil error, want error")
	}
}

// TestAddSessionRollsBackOnDuplicate proves AddSession is atomic: a failing
// insert inside the transaction leaves the project's counters untouched.
func TestAddSessionRollsBackOnDuplicate(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	if err := svc.AddSession("p1", "s1", "Session 1", "", "", 2); err != nil {
		t.Fatalf("AddSession: %v", err)
	}

	if err := svc.AddSession("p1", "s1", "dup", "", "", 9); err == nil {
		t.Fatal("duplicate session id = nil error, want error")
	}
	projects, _ := svc.LoadState()
	if projects[0].NextSeq != 2 || projects[0].ActiveSessionID != "s1" {
		t.Errorf("counters changed after rollback: next_seq=%d active=%q",
			projects[0].NextSeq, projects[0].ActiveSessionID)
	}
}

// TestOperationsOnClosedStoreReturnErrors proves every persistence call surfaces
// the underlying error instead of swallowing it.
func TestOperationsOnClosedStoreReturnErrors(t *testing.T) {
	svc, err := open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	if err := svc.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	assertErr := func(name string, err error) {
		t.Helper()
		if err == nil {
			t.Errorf("%s on closed store = nil error, want error", name)
		}
	}
	assertErr("AddProject", svc.AddProject("p2", "b", "/b"))
	assertErr("CloseProject", svc.CloseProject("p1"))
	assertErr("AddSession", svc.AddSession("p1", "s1", "Session 1", "", "", 2))
	assertErr("DeleteSession", svc.DeleteSession("p1", "s1", ""))
	assertErr("RenameSession", svc.RenameSession("s1", "x"))
	assertErr("SetClaudeSession", svc.SetClaudeSession("s1", "uuid"))
	assertErr("SetActiveSession", svc.SetActiveSession("p1", "s1"))
	if _, err := svc.SetSessionTitle("s1", "x"); err == nil {
		t.Error("SetSessionTitle on closed store = nil error, want error")
	}
	assertErr("SetSetting", svc.SetSetting("k", "", "v"))

	if _, err := svc.GetSetting("k", ""); err == nil {
		t.Error("GetSetting on closed store = nil error, want error")
	}
	if _, err := svc.LoadState(); err == nil {
		t.Error("LoadState on closed store = nil error, want error")
	}
	if got := svc.ClaudeBin("p1"); got != "" {
		t.Errorf("ClaudeBin on closed store = %q, want empty", got)
	}
}

func TestDeleteProjectCascadesSessions(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddSession("p1", "s1", "Session 1", "", "", 2)

	// Direct row delete exercises the ON DELETE CASCADE foreign key (the
	// future "forget project" path); close does not delete, so it is not used
	// here.
	if _, err := svc.db.Exec(`DELETE FROM projects WHERE id = ?`, "p1"); err != nil {
		t.Fatalf("delete project: %v", err)
	}
	var count int
	if err := svc.db.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&count); err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	if count != 0 {
		t.Errorf("orphan sessions after cascade = %d, want 0", count)
	}
}

// sessionIDs returns a project's session ids in stored order.
func sessionIDs(t *testing.T, svc *Service, projectID string) []string {
	t.Helper()
	sessions, err := svc.sessionsOf(projectID)
	if err != nil {
		t.Fatalf("sessionsOf: %v", err)
	}
	ids := make([]string, len(sessions))
	for i, s := range sessions {
		ids[i] = s.ID
	}
	return ids
}

// openProjectIDs returns the open project ids in stored order.
func openProjectIDs(t *testing.T, svc *Service) []string {
	t.Helper()
	projects, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	ids := make([]string, len(projects))
	for i, p := range projects {
		ids[i] = p.ID
	}
	return ids
}

func equalIDs(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestReorderSessionsPersistsOrder(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddSession("p1", "s1", "Session 1", "", "", 2)
	_ = svc.AddSession("p1", "s2", "Session 2", "", "", 3)
	_ = svc.AddSession("p1", "s3", "Session 3", "", "", 4)

	if err := svc.ReorderSessions("p1", []string{"s3", "s1", "s2"}); err != nil {
		t.Fatalf("ReorderSessions: %v", err)
	}
	if got := sessionIDs(t, svc, "p1"); !equalIDs(got, []string{"s3", "s1", "s2"}) {
		t.Errorf("session order = %v, want [s3 s1 s2]", got)
	}
}

// A session opened after a drag must land at the end of the card list, not at
// the top where the default position would put it.
func TestAddSessionAppendsAfterReorder(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddSession("p1", "s1", "Session 1", "", "", 2)
	_ = svc.AddSession("p1", "s2", "Session 2", "", "", 3)
	_ = svc.ReorderSessions("p1", []string{"s2", "s1"})

	_ = svc.AddSession("p1", "s3", "Session 3", "", "", 4)

	if got := sessionIDs(t, svc, "p1"); !equalIDs(got, []string{"s2", "s1", "s3"}) {
		t.Errorf("session order = %v, want [s2 s1 s3]", got)
	}
}

// A session id from another project must not take a position in this one.
func TestReorderSessionsIgnoresForeignSession(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddProject("p2", "beta", "/tmp/beta")
	_ = svc.AddSession("p1", "s1", "Session 1", "", "", 2)
	_ = svc.AddSession("p1", "s2", "Session 2", "", "", 3)
	_ = svc.AddSession("p2", "other", "Session 1", "", "", 2)

	if err := svc.ReorderSessions("p1", []string{"other", "s2", "s1"}); err != nil {
		t.Fatalf("ReorderSessions: %v", err)
	}
	if got := sessionIDs(t, svc, "p2"); !equalIDs(got, []string{"other"}) {
		t.Errorf("p2 sessions = %v, want [other] untouched", got)
	}
	if got := sessionIDs(t, svc, "p1"); !equalIDs(got, []string{"s2", "s1"}) {
		t.Errorf("p1 order = %v, want [s2 s1]", got)
	}
}

func TestReorderProjectsPersistsOrder(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddProject("p2", "beta", "/tmp/beta")
	_ = svc.AddProject("p3", "gamma", "/tmp/gamma")

	if err := svc.ReorderProjects([]string{"p3", "p1", "p2"}); err != nil {
		t.Fatalf("ReorderProjects: %v", err)
	}
	if got := openProjectIDs(t, svc); !equalIDs(got, []string{"p3", "p1", "p2"}) {
		t.Errorf("project order = %v, want [p3 p1 p2]", got)
	}
}

// A project opened after a drag becomes the rightmost tab.
func TestAddProjectAppendsAfterReorder(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddProject("p2", "beta", "/tmp/beta")
	_ = svc.ReorderProjects([]string{"p2", "p1"})

	_ = svc.AddProject("p3", "gamma", "/tmp/gamma")

	if got := openProjectIDs(t, svc); !equalIDs(got, []string{"p2", "p1", "p3"}) {
		t.Errorf("project order = %v, want [p2 p1 p3]", got)
	}
}

// Reopening a closed project restores it to the tab slot it was dragged to,
// rather than moving it to the end.
func TestReopenProjectKeepsDraggedPosition(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddProject("p2", "beta", "/tmp/beta")
	_ = svc.AddProject("p3", "gamma", "/tmp/gamma")
	_ = svc.ReorderProjects([]string{"p3", "p1", "p2"})
	_ = svc.CloseProject("p1")

	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")

	if got := openProjectIDs(t, svc); !equalIDs(got, []string{"p3", "p1", "p2"}) {
		t.Errorf("project order = %v, want [p3 p1 p2]", got)
	}
}

// TestOpenMigratesPreMigrationDatabase proves open() applies the ADD COLUMN
// migrations to a database created before those columns existed — the path
// where a migration actually succeeds (err == nil). A current-schema database
// never reaches it: every ALTER there is a duplicate-column no-op, which is why
// only this test kills the mutant that flips migrate's `err != nil` to
// `err == nil` (that inversion dereferences a nil error here and panics).
func TestOpenMigratesPreMigrationDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")

	// The schema as it stood before the migrations: sessions without
	// kind/path/claude_session_id/label_auto/position, projects without
	// position.
	const oldSchema = `
CREATE TABLE projects (
    id                TEXT    PRIMARY KEY,
    name              TEXT    NOT NULL,
    path              TEXT    NOT NULL,
    is_open           INTEGER NOT NULL DEFAULT 1,
    next_seq          INTEGER NOT NULL DEFAULT 1,
    active_session_id TEXT    NOT NULL DEFAULT ''
);
CREATE TABLE sessions (
    id         TEXT NOT NULL PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    label      TEXT NOT NULL
);`

	seed, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open seed db: %v", err)
	}
	if _, err := seed.Exec(oldSchema); err != nil {
		t.Fatalf("seed old schema: %v", err)
	}
	_ = seed.Close()

	svc, err := open(path)
	if err != nil {
		t.Fatalf("open pre-migration db: %v", err)
	}
	t.Cleanup(func() { _ = svc.Close() })

	// The columns the migrations added must now be usable end to end.
	if err := svc.AddProject("p1", "alpha", "/tmp/alpha"); err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	if err := svc.AddSession("p1", "s1", "mellow-otter", "shell", "/data/wt", 2); err != nil {
		t.Fatalf("AddSession: %v", err)
	}
	got, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if len(got) != 1 || len(got[0].Sessions) != 1 {
		t.Fatalf("state after migration = %+v", got)
	}
	if s := got[0].Sessions[0]; s.Kind != "shell" || s.Path != "/data/wt" {
		t.Errorf("migrated session = %+v, want kind=shell path=/data/wt", s)
	}
}
