package spawn

import (
	"errors"
	"os"
	"regexp"
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

// A session opened into a folder carries it from the insert, before the window
// hears of it, so the card arrives in the folder's block rather than among its
// checkout's cards and then jumps, and no crash can land between the row and a
// second filing write.
func TestOpenFilesTheSessionWithItsInsert(t *testing.T) {
	svc, sessions, _, _, events := newService(t)

	opened, err := svc.Open(OpenOptions{From: "s1", Folder: " Apps "})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if opened.Folder != "Apps" || len(sessions.rows) != 1 || sessions.rows[0].folder != "Apps" {
		t.Errorf("folder = %q, rows = %+v; want the trimmed name on both", opened.Folder, sessions.rows)
	}
	if len(sessions.folders) != 0 {
		t.Errorf("filed %v after the insert, want no second write", sessions.folders)
	}
	if len(events.events) != 1 || events.events[0].data.(Session).Folder != "Apps" {
		t.Errorf("events = %+v, want the card announced already filed", events.events)
	}
}

func TestOpenWithoutAFolderFilesNothing(t *testing.T) {
	svc, sessions, _, _, _ := newService(t)

	if _, err := svc.Open(OpenOptions{From: "s1"}); err != nil {
		t.Fatalf("Open: %v", err)
	}
	if len(sessions.rows) != 1 || sessions.rows[0].folder != "" || len(sessions.folders) != 0 {
		t.Errorf("rows %+v, filed %v; want the folder column left alone", sessions.rows, sessions.folders)
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

// Painting a folder paints each card filed under it, in that project alone, and
// the window hears which cards took the colour.
func TestColorFolderPaintsEverySessionInIt(t *testing.T) {
	svc, sessions, events := filer(t)

	colored, err := svc.ColorFolder("s1", "", " Apps ", " Teal ")
	if err != nil {
		t.Fatalf("ColorFolder: %v", err)
	}
	if !slices.Equal(sessions.colored, [][3]string{{"p1", "Apps", "teal"}}) {
		t.Errorf("colored = %v, want the one project's folder painted teal", sessions.colored)
	}
	if !slices.Equal(colored.Sessions, []string{"shared-a", "alone"}) || colored.Color != "teal" {
		t.Errorf("colored = %+v, want every painted card named", colored)
	}
	if len(events.events) != 1 || events.events[0].name != ColoredEventName {
		t.Fatalf("events = %+v, want one %s", events.events, ColoredEventName)
	}
	got := events.events[0].data.(ColoredEvent)
	if got.ProjectID != "p1" || !slices.Equal(got.IDs, []string{"s2", "s4"}) || got.Color != "teal" {
		t.Errorf("event = %+v, want both cards painted and none of revu's", got)
	}
}

// An empty colour hands the folder's cards back to the theme.
func TestColorFolderWithNoColorClearsIt(t *testing.T) {
	svc, sessions, _ := filer(t)

	if _, err := svc.ColorFolder("s1", "", "Infra", ""); err != nil {
		t.Fatalf("ColorFolder: %v", err)
	}
	if !slices.Equal(sessions.colored, [][3]string{{"p1", "Infra", ""}}) {
		t.Errorf("colored = %v, want the folder cleared", sessions.colored)
	}
}

// The window draws a name outside its palette as no colour at all, so a caller
// told its paint landed would be wrong: the refusal lists the palette instead.
func TestColorFolderRefusesAColorOutsideThePalette(t *testing.T) {
	svc, sessions, events := filer(t)

	_, err := svc.ColorFolder("s1", "", "Apps", "purple")
	if err == nil {
		t.Fatal("painted a folder with a colour the window cannot draw")
	}
	for _, phrase := range []string{`"purple"`, "violet", "teal"} {
		if !strings.Contains(err.Error(), phrase) {
			t.Errorf("error = %q, want it to mention %s", err, phrase)
		}
	}
	if len(sessions.colored) != 0 || len(events.events) != 0 {
		t.Error("wrote something for a colour that was refused")
	}
}

// A name no session carries is refused for RenameFolder's reason: the write
// would match nothing and answer success.
func TestColorFolderRefusesANameNoSessionCarries(t *testing.T) {
	svc, sessions, events := filer(t)

	_, err := svc.ColorFolder("s1", "", "apps", "red")
	if err == nil || !strings.Contains(err.Error(), "Apps") {
		t.Fatalf("err = %v, want a refusal naming the folders there are", err)
	}
	if len(sessions.colored) != 0 || len(events.events) != 0 {
		t.Error("wrote something for a folder that is not there")
	}
}

func TestColorFolderThatCannotBeWrittenAnnouncesNothing(t *testing.T) {
	svc, sessions, events := filer(t)
	sessions.folderErr = errors.New("disk full")

	if _, err := svc.ColorFolder("s1", "", "Apps", "red"); err == nil {
		t.Fatal("ColorFolder = nil, want the write's error")
	}
	if len(events.events) != 0 {
		t.Errorf("events = %+v, want nothing announced", events.events)
	}
}

// The palette's values live in the window; this list only has to name the same
// colours, or a name the window offers would be refused here and one it lacks
// accepted and drawn as nothing.
func TestCardColorsNameTheWindowsPalette(t *testing.T) {
	src, err := os.ReadFile("../../frontend/src/lib/session/card-color.ts")
	if err != nil {
		t.Fatalf("read the window's palette: %v", err)
	}
	var names []string
	for _, m := range regexp.MustCompile(`(?m)^\s+(\w+): "oklch\(`).FindAllStringSubmatch(string(src), -1) {
		names = append(names, m[1])
	}
	if !slices.Equal(names, cardColors) {
		t.Errorf("window palette %v, cardColors %v", names, cardColors)
	}
}
