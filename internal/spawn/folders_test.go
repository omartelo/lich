package spawn

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/omartelo/lich/internal/store"
)

// filedWorkspace is closable with folders on it: two sessions in "Apps", one in
// "Infra", one unfiled, and a second project holding an "Apps" of its own;
// the name is scoped by project, so nothing here may reach into it.
func filedWorkspace() []store.Project {
	projects := closable()
	projects[0].Sessions[1].Folder = "Apps"
	projects[0].Sessions[2].Folder = "Infra"
	projects[0].Sessions[3].Folder = "Apps"
	return append(projects, store.Project{
		ID: "p2", Name: "revu", Path: "/src/revu", NextSeq: 2,
		Sessions: []store.Session{{ID: "o1", Label: "shared-a", Kind: "claude", Folder: "Apps"}},
	})
}

func filer(t *testing.T) (*Service, *fakeSessions, *fakeEvents) {
	t.Helper()
	svc, sessions, _, _, events := closer(t)
	sessions.projects = filedWorkspace()
	return svc, sessions, events
}

// filedEvent reads the one sessions-filed announcement a call made.
func filedEvent(t *testing.T, events *fakeEvents) FiledEvent {
	t.Helper()
	if len(events.events) != 1 || events.events[0].name != FiledEventName {
		t.Fatalf("events = %+v, want one %s", events.events, FiledEventName)
	}
	return events.events[0].data.(FiledEvent)
}

// A session opened into a folder is filed before the window hears of it, so
// the card arrives in the folder's block rather than among its checkout's cards
// and then jumps.
func TestOpenFilesTheSessionBeforeTheCardIsAnnounced(t *testing.T) {
	svc, sessions, _, _, events := newService(t)

	opened, err := svc.Open("s1", "", "", "", "", "", "", " Apps ", false)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if opened.Folder != "Apps" || sessions.folders[opened.ID] != "Apps" {
		t.Errorf("folder = %q, row = %v; want the trimmed name on both", opened.Folder, sessions.folders)
	}
	if len(events.events) != 1 || events.events[0].data.(Session).Folder != "Apps" {
		t.Errorf("events = %+v, want the card announced already filed", events.events)
	}
}

func TestOpenWithoutAFolderFilesNothing(t *testing.T) {
	svc, sessions, _, _, _ := newService(t)

	if _, err := svc.Open("s1", "", "", "", "", "", "", "", false); err != nil {
		t.Fatalf("Open: %v", err)
	}
	if len(sessions.folders) != 0 {
		t.Errorf("filed %v, want the folder column left alone", sessions.folders)
	}
}

// A filing that cannot be written costs the folder and nothing more: the row is
// there, so the card and its terminal have to be too, and the caller hears
// which part was lost.
func TestOpenStartsTheSessionEvenWhenItCannotBeFiled(t *testing.T) {
	svc, sessions, _, term, events := newService(t)
	sessions.folderErr = errors.New("database is locked")

	_, err := svc.Open("s1", "", "", "", "", "", "", "Apps", false)
	if err == nil || !strings.Contains(err.Error(), `"Apps"`) {
		t.Fatalf("Open = %v, want the lost folder named", err)
	}
	if len(term.spawns) != 1 {
		t.Errorf("started %d terminals, want the session running anyway", len(term.spawns))
	}
	if len(events.events) != 1 || events.events[0].data.(Session).Folder != "" {
		t.Errorf("events = %+v, want the card announced unfiled, as the row is", events.events)
	}
}

// Folders come in the order their first card sits in the stored list (the
// order the sidebar draws their blocks in), each naming its sessions.
func TestFoldersListsEachFolderWithItsSessions(t *testing.T) {
	svc, _, _ := filer(t)

	folders, err := svc.Folders("s1", "")
	if err != nil {
		t.Fatalf("Folders: %v", err)
	}
	want := []Folder{
		{Name: "Apps", Sessions: []string{"shared-a", "alone"}},
		{Name: "Infra", Sessions: []string{"shared-b"}},
	}
	if !slices.EqualFunc(folders, want, func(a, b Folder) bool {
		return a.Name == b.Name && slices.Equal(a.Sessions, b.Sessions)
	}) {
		t.Errorf("folders = %+v, want %+v", folders, want)
	}
}

func TestFoldersOfAProjectWithNoneIsAnEmptyList(t *testing.T) {
	svc, sessions, _ := filer(t)
	sessions.projects[0].Sessions = closable()[0].Sessions

	folders, err := svc.Folders("s1", "lich")
	if err != nil {
		t.Fatalf("Folders: %v", err)
	}
	if folders == nil || len(folders) != 0 {
		t.Errorf("folders = %#v, want an empty list a script need not tell from null", folders)
	}
}

func TestFileMovesASessionIntoAFolderAndAnnouncesIt(t *testing.T) {
	svc, sessions, events := filer(t)

	filed, err := svc.File("s1", "shared-b", "", " Apps ")
	if err != nil {
		t.Fatalf("File: %v", err)
	}
	if filed.Label != "shared-b" || filed.Folder != "Apps" || filed.Previous != "Infra" {
		t.Errorf("filed = %+v, want both ends of the move", filed)
	}
	if sessions.folders["s3"] != "Apps" {
		t.Errorf("folders = %v, want the target written under the trimmed name", sessions.folders)
	}
	if got := filedEvent(t, events); got.ProjectID != "p1" || !slices.Equal(got.IDs, []string{"s3"}) ||
		got.Folder != "Apps" {
		t.Errorf("event = %+v, want the one card moved into Apps", got)
	}
}

// The caller's own session is the one an agent reaches without discovery, the
// same as a rename.
func TestFileWithoutATargetFilesTheCaller(t *testing.T) {
	svc, sessions, _ := filer(t)

	if _, err := svc.File("s1", "", "", "Planning"); err != nil {
		t.Fatalf("File: %v", err)
	}
	if sessions.folders["s1"] != "Planning" {
		t.Errorf("folders = %v, want the calling session filed", sessions.folders)
	}
}

// An empty folder is the window's "Take out of": the same write, with no name.
func TestFileIntoNoFolderTakesTheSessionOut(t *testing.T) {
	svc, sessions, events := filer(t)

	filed, err := svc.File("s1", "alone", "", "  ")
	if err != nil {
		t.Fatalf("File: %v", err)
	}
	if filed.Folder != "" || filed.Previous != "Apps" {
		t.Errorf("filed = %+v, want it out of Apps", filed)
	}
	if folder, written := sessions.folders["s4"]; !written || folder != "" {
		t.Errorf("folders = %v, want the target's folder cleared", sessions.folders)
	}
	if got := filedEvent(t, events); got.Folder != "" {
		t.Errorf("event = %+v, want the card told it is in no folder", got)
	}
}

func TestFileOutsideASessionNeedsATarget(t *testing.T) {
	svc, _, _ := filer(t)

	_, err := svc.File("", "", "", "Apps")
	if err == nil || !strings.Contains(err.Error(), "name the session") {
		t.Fatalf("File = %v, want it to say what the caller must do", err)
	}
}

// A failed write leaves the card alone, for the rename's reason: a window told
// the card moved over a row that did not would show a folder the next reload
// takes away.
func TestFileThatCannotBeWrittenAnnouncesNothing(t *testing.T) {
	svc, sessions, events := filer(t)
	sessions.folderErr = errors.New("disk is gone")

	if _, err := svc.File("s1", "alone", "", "Infra"); err == nil {
		t.Fatal("reported a filing the store refused")
	}
	if len(events.events) != 0 {
		t.Errorf("events = %+v, want none for a filing that was not written", events.events)
	}
}

// A rename moves every session carrying the name, and only in its project: the
// revu project's own "Apps" is another folder.
func TestRenameFolderMovesEverySessionInIt(t *testing.T) {
	svc, sessions, events := filer(t)

	refiled, err := svc.RenameFolder("s1", "", "Apps", " Applications ")
	if err != nil {
		t.Fatalf("RenameFolder: %v", err)
	}
	if !slices.Equal(sessions.refolded, [][3]string{{"p1", "Apps", "Applications"}}) {
		t.Errorf("refolded = %v, want the one project's folder renamed", sessions.refolded)
	}
	if !slices.Equal(refiled.Sessions, []string{"shared-a", "alone"}) || refiled.To != "Applications" {
		t.Errorf("refiled = %+v, want every card that moved named", refiled)
	}
	if got := filedEvent(t, events); got.ProjectID != "p1" || !slices.Equal(got.IDs, []string{"s2", "s4"}) ||
		got.Folder != "Applications" {
		t.Errorf("event = %+v, want both cards moved and none of revu's", got)
	}
}

// A card the window filed under the old name after this call read the
// workspace is still rewritten by the store, which matches by name. The event
// and the answer name what the write moved, or that card stays drawn in a
// folder that no longer exists until the window reloads.
func TestRenameFolderAnnouncesWhatTheWriteMoved(t *testing.T) {
	svc, sessions, events := filer(t)
	sessions.moved = []string{"s2", "s3", "s4"}

	refiled, err := svc.RenameFolder("s1", "", "Apps", "Applications")
	if err != nil {
		t.Fatalf("RenameFolder: %v", err)
	}
	if got := filedEvent(t, events); !slices.Equal(got.IDs, []string{"s2", "s3", "s4"}) {
		t.Errorf("event ids = %v, want every row the store rewrote", got.IDs)
	}
	if !slices.Equal(refiled.Sessions, []string{"shared-a", "shared-b", "alone"}) {
		t.Errorf("refiled = %v, want the card filed in between named too", refiled.Sessions)
	}
}

// An empty new name is the window's Ungroup: every card goes back to its
// checkout's block.
func TestRenameFolderToNothingTakesItApart(t *testing.T) {
	svc, sessions, events := filer(t)

	if _, err := svc.RenameFolder("s1", "", "Infra", ""); err != nil {
		t.Fatalf("RenameFolder: %v", err)
	}
	if !slices.Equal(sessions.refolded, [][3]string{{"p1", "Infra", ""}}) {
		t.Errorf("refolded = %v, want the folder taken apart", sessions.refolded)
	}
	if got := filedEvent(t, events); got.Folder != "" || !slices.Equal(got.IDs, []string{"s3"}) {
		t.Errorf("event = %+v, want the card told it is in no folder", got)
	}
}

// The store matches no row for a name nothing carries and answers success; a
// caller told its rename landed would go on to address a folder that is not
// there. The refusal lists the ones that are, since the likely cause is a typo
// or a case the store does not fold.
func TestRenameFolderRefusesANameNoSessionCarries(t *testing.T) {
	svc, sessions, events := filer(t)

	_, err := svc.RenameFolder("s1", "", "apps", "Applications")
	if err == nil {
		t.Fatal("renamed a folder no session is filed under")
	}
	for _, phrase := range []string{`"apps"`, "Apps", "Infra"} {
		if !strings.Contains(err.Error(), phrase) {
			t.Errorf("error = %q, want it to mention %s", err, phrase)
		}
	}
	if len(sessions.refolded) != 0 || len(events.events) != 0 {
		t.Error("wrote something for a rename that was refused")
	}
}

// An empty name would match every unfiled session and sweep them all into the
// new folder.
func TestRenameFolderRefusesAnEmptyName(t *testing.T) {
	svc, sessions, _ := filer(t)

	if _, err := svc.RenameFolder("s1", "", " ", "Everything"); err == nil {
		t.Fatal("renamed the unfiled sessions as if they were a folder")
	}
	if len(sessions.refolded) != 0 {
		t.Error("wrote a rename of no folder")
	}
}

func TestRenameFolderThatCannotBeWrittenAnnouncesNothing(t *testing.T) {
	svc, sessions, events := filer(t)
	sessions.folderErr = errors.New("disk is gone")

	if _, err := svc.RenameFolder("s1", "", "Apps", "Applications"); err == nil {
		t.Fatal("reported a rename the store refused")
	}
	if len(events.events) != 0 {
		t.Errorf("events = %+v, want none for a rename that was not written", events.events)
	}
}

// A project with no folders at all gets the refusal that says so, rather than an
// empty list of names to pick from.
func TestRenameFolderInAProjectWithNoFoldersSaysSo(t *testing.T) {
	svc, sessions, _ := filer(t)
	sessions.projects[0].Sessions = closable()[0].Sessions

	_, err := svc.RenameFolder("s1", "", "Apps", "Applications")
	if err == nil || !strings.Contains(err.Error(), "It has no folders.") {
		t.Fatalf("err = %v, want the refusal to say the project has no folders", err)
	}
	if len(sessions.refolded) != 0 {
		t.Error("wrote a rename in a project with no folders")
	}
}

// A project no one has open is refused by every folder call before anything is
// read off it or written to it.
func TestFolderCallsRefuseAnUnknownProject(t *testing.T) {
	svc, sessions, events := filer(t)

	if _, err := svc.Folders("s1", "nowhere"); err == nil {
		t.Error("Folders listed a project that is not open")
	}
	if _, err := svc.RenameFolder("s1", "nowhere", "Apps", "Applications"); err == nil {
		t.Error("RenameFolder renamed in a project that is not open")
	}
	if len(sessions.refolded) != 0 || len(events.events) != 0 {
		t.Error("wrote something for a project that is not open")
	}
}

// A workspace that cannot be read leaves every folder call with nothing to act
// on, and the error says it was the read that failed.
func TestFolderCallsReportAWorkspaceThatCannotBeRead(t *testing.T) {
	svc, sessions, events := filer(t)
	sessions.loadErr = errors.New("database is locked")

	calls := map[string]func() error{
		"Folders": func() error { _, err := svc.Folders("s1", ""); return err },
		"File":    func() error { _, err := svc.File("s1", "alone", "", "Infra"); return err },
		"RenameFolder": func() error {
			_, err := svc.RenameFolder("s1", "", "Apps", "Applications")
			return err
		},
	}
	for name, call := range calls {
		if err := call(); err == nil || !strings.Contains(err.Error(), "read the workspace") {
			t.Errorf("%s err = %v, want the failed read named", name, err)
		}
	}
	if len(sessions.folders) != 0 || len(sessions.refolded) != 0 || len(events.events) != 0 {
		t.Error("wrote something without having read the workspace")
	}
}
