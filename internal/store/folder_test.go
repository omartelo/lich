package store

import "testing"

// folderOf reads one session's folder straight off the row, so a test asserting
// where a session was filed never depends on how the sidebar groups them.
func folderOf(t *testing.T, svc *Service, projectID, sessionID string) string {
	t.Helper()
	sessions, err := svc.sessionsOf(projectID)
	if err != nil {
		t.Fatalf("sessionsOf: %v", err)
	}
	for _, s := range sessions {
		if s.ID == sessionID {
			return s.Folder
		}
	}
	t.Fatalf("session %q not in project %q", sessionID, projectID)
	return ""
}

// TestSetSessionFolderFilesAndUnfiles: a folder is an assignment on the session,
// so filing one is a write and taking it out is the same write with an empty
// name — never a second concept the sidebar would have to tell apart.
func TestSetSessionFolderFilesAndUnfiles(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddSession("p1", "s1", "Session 1", "", "", 2, "")

	if err := svc.SetSessionFolder("s1", "Design system"); err != nil {
		t.Fatalf("SetSessionFolder: %v", err)
	}
	if got := folderOf(t, svc, "p1", "s1"); got != "Design system" {
		t.Errorf("folder = %q, want %q", got, "Design system")
	}
	if err := svc.SetSessionFolder("s1", ""); err != nil {
		t.Fatalf("SetSessionFolder(unfile): %v", err)
	}
	if got := folderOf(t, svc, "p1", "s1"); got != "" {
		t.Errorf("folder after unfiling = %q, want empty", got)
	}
}

// TestSetSessionFolderLeavesTheDragOrder: filing a card must not move it. The
// sidebar lifts a filed card into its block at render time, exactly as it does
// a pinned one, so the stored order is what an unfiled card falls back into.
func TestSetSessionFolderLeavesTheDragOrder(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddSession("p1", "s1", "Session 1", "", "", 2, "")
	_ = svc.AddSession("p1", "s2", "Session 2", "", "", 3, "")
	_ = svc.AddSession("p1", "s3", "Session 3", "", "", 4, "")
	_ = svc.ReorderSessions("p1", []string{"s3", "s1", "s2"})

	if err := svc.SetSessionFolder("s1", "Apps"); err != nil {
		t.Fatalf("SetSessionFolder: %v", err)
	}
	if got := sessionIDs(t, svc, "p1"); !equalIDs(got, []string{"s3", "s1", "s2"}) {
		t.Errorf("order after filing = %v, want [s3 s1 s2]", got)
	}
}

// TestRenameFolderRewritesItsSessionsOnly: the name is the folder's identity, so
// a rename is a write across the sessions carrying it — and it stops at the
// project, because two projects can hold folders of the same name.
func TestRenameFolderRewritesItsSessionsOnly(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddProject("p2", "beta", "/tmp/beta")
	_ = svc.AddSession("p1", "s1", "Session 1", "", "", 2, "")
	_ = svc.AddSession("p1", "s2", "Session 2", "", "", 3, "")
	_ = svc.AddSession("p2", "o1", "Other 1", "", "", 2, "")
	_ = svc.SetSessionFolder("s1", "Apps")
	_ = svc.SetSessionFolder("s2", "Infra")
	_ = svc.SetSessionFolder("o1", "Apps")

	if err := svc.RenameFolder("p1", "Apps", "Applications"); err != nil {
		t.Fatalf("RenameFolder: %v", err)
	}
	if got := folderOf(t, svc, "p1", "s1"); got != "Applications" {
		t.Errorf("renamed folder = %q, want %q", got, "Applications")
	}
	if got := folderOf(t, svc, "p1", "s2"); got != "Infra" {
		t.Errorf("other folder = %q, want %q", got, "Infra")
	}
	if got := folderOf(t, svc, "p2", "o1"); got != "Apps" {
		t.Errorf("other project's folder = %q, want %q", got, "Apps")
	}
}

// TestRenameFolderToNothingUngroups: taking a folder apart is the rename with an
// empty target — one write, and every card falls back to its checkout's block.
func TestRenameFolderToNothingUngroups(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddSession("p1", "s1", "Session 1", "", "", 2, "")
	_ = svc.AddSession("p1", "s2", "Session 2", "", "", 3, "")
	_ = svc.SetSessionFolder("s1", "Apps")
	_ = svc.SetSessionFolder("s2", "Apps")

	if err := svc.RenameFolder("p1", "Apps", ""); err != nil {
		t.Fatalf("RenameFolder: %v", err)
	}
	for _, id := range []string{"s1", "s2"} {
		if got := folderOf(t, svc, "p1", id); got != "" {
			t.Errorf("session %q folder = %q, want empty", id, got)
		}
	}
}

// TestRenameFolderRefusesTheEmptyName: without the guard, renaming "" would
// sweep every unfiled session in the project into a folder nobody asked for.
func TestRenameFolderRefusesTheEmptyName(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddSession("p1", "s1", "Session 1", "", "", 2, "")

	if err := svc.RenameFolder("p1", "", "Apps"); err == nil {
		t.Fatal("RenameFolder with no source folder = nil, want an error")
	}
	if got := folderOf(t, svc, "p1", "s1"); got != "" {
		t.Errorf("unfiled session folder = %q, want empty", got)
	}
}

// TestResumeKeepsTheFolder: where a session was filed is not something closing
// it undoes. A resume that dropped the folder would land the card back among its
// checkout's own — the pile the folder was made to break up.
func TestResumeKeepsTheFolder(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddSession("p1", "s1", "Session 1", "", "", 2, "")
	_ = svc.SetSessionFolder("s1", "Design system")
	if err := svc.CloseSession("p1", "s1", ""); err != nil {
		t.Fatalf("CloseSession: %v", err)
	}

	restored, err := svc.ReopenSession("s1", "s1-new")
	if err != nil {
		t.Fatalf("ReopenSession: %v", err)
	}
	if restored == nil {
		t.Fatal("ReopenSession returned no session")
	}
	if restored.Folder != "Design system" {
		t.Errorf("resumed folder = %q, want %q", restored.Folder, "Design system")
	}
	if got := folderOf(t, svc, "p1", "s1-new"); got != "Design system" {
		t.Errorf("stored folder after resume = %q, want %q", got, "Design system")
	}
}

// TestRenameFolderReachesParkedSessions: a parked row is resumed into the folder
// it is filed under, so a rename while it is away has to find it — otherwise the
// session comes back into a folder the user renamed out of existence.
func TestRenameFolderReachesParkedSessions(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddSession("p1", "s1", "Session 1", "", "", 2, "")
	_ = svc.SetSessionFolder("s1", "Apps")
	_ = svc.CloseSession("p1", "s1", "")

	if err := svc.RenameFolder("p1", "Apps", "Applications"); err != nil {
		t.Fatalf("RenameFolder: %v", err)
	}
	restored, err := svc.ReopenSession("s1", "s1-new")
	if err != nil {
		t.Fatalf("ReopenSession: %v", err)
	}
	if restored == nil {
		t.Fatal("ReopenSession returned no session")
	}
	if restored.Folder != "Applications" {
		t.Errorf("resumed folder = %q, want %q", restored.Folder, "Applications")
	}
}
