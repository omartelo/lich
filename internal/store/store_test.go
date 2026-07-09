package store

import (
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
	if err := svc.AddSession("p1", "s1", "Session 1", 2); err != nil {
		t.Fatalf("AddSession: %v", err)
	}
	if err := svc.AddSession("p1", "s2", "Session 2", 3); err != nil {
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

func TestCloseProjectHidesButKeepsSessions(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddSession("p1", "s1", "Session 1", 2)

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
	_ = svc.AddSession("p1", "s1", "Session 1", 2)
	_ = svc.AddSession("p1", "s2", "Session 2", 3)

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
	_ = svc.AddSession("p1", "s1", "Session 1", 2)
	_ = svc.AddSession("p1", "s2", "Session 2", 3)

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

func TestDatabasePath(t *testing.T) {
	path, err := databasePath()
	if err != nil {
		t.Fatalf("databasePath: %v", err)
	}
	if filepath.Base(path) != "skipo.db" || filepath.Base(filepath.Dir(path)) != "skipo" {
		t.Errorf("path = %q, want .../skipo/skipo.db", path)
	}
}

func TestOpenFailsWhenParentIsAFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	// The db's parent directory cannot be created under an existing file.
	if _, err := open(filepath.Join(file, "skipo.db")); err == nil {
		t.Error("open under a file parent = nil error, want error")
	}
}

// TestAddSessionRollsBackOnDuplicate proves AddSession is atomic: a failing
// insert inside the transaction leaves the project's counters untouched.
func TestAddSessionRollsBackOnDuplicate(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	if err := svc.AddSession("p1", "s1", "Session 1", 2); err != nil {
		t.Fatalf("AddSession: %v", err)
	}

	if err := svc.AddSession("p1", "s1", "dup", 9); err == nil {
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
	assertErr("AddSession", svc.AddSession("p1", "s1", "Session 1", 2))
	assertErr("DeleteSession", svc.DeleteSession("p1", "s1", ""))
	assertErr("RenameSession", svc.RenameSession("s1", "x"))
	assertErr("SetActiveSession", svc.SetActiveSession("p1", "s1"))
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
	_ = svc.AddSession("p1", "s1", "Session 1", 2)

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
