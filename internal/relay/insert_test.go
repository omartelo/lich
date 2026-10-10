package relay

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/omartelo/lich/internal/store"
)

// editorWorkspace is a project with sessions in its root checkout and in a
// worktree, which is where an editor's open folder has to find them.
func editorWorkspace(root, worktree string) fakeSessions {
	return fakeSessions{projects: []store.Project{
		{ID: "p1", Name: "lich", Path: root, Sessions: []store.Session{
			{ID: "s1", Label: "main", Kind: "claude"},
			{ID: "s2", Label: "feature", Kind: "codex", Path: worktree},
		}},
		{ID: "p2", Name: "revu", Path: filepath.Join(root, "other"), Sessions: []store.Session{
			{ID: "s3", Label: "solo", Kind: "crush"},
		}},
	}}
}

func TestInsertPastesWithoutSubmitting(t *testing.T) {
	term := newFakeTerminal("s1", "s2", "s3")
	svc := newRelay(editorWorkspace("/src/lich", "/src/lich-wt"), term, nil)

	got, err := svc.Insert("", "solo", "", "func main() {}\n", 0)
	if err != nil {
		t.Fatalf("Insert = %v, want nil", err)
	}
	if got.ID != "s3" || got.Label != "solo" || got.Kind != "crush" || got.Bytes != len("func main() {}\n") {
		t.Errorf("Insert = %+v, want the session it landed in", got)
	}
	writes := term.writesTo("s3")
	if len(writes) != 1 || writes[0] != "\x1b[200~func main() {}\n\x1b[201~" {
		t.Errorf("writes = %q, want one bracketed paste and no Enter", writes)
	}
}

func TestInsertStripsWhatWouldBreakOutOfThePaste(t *testing.T) {
	term := newFakeTerminal("s3")
	svc := newRelay(editorWorkspace("/src/lich", "/src/lich-wt"), term, nil)

	if _, err := svc.Insert("", "solo", "", "a\x1b[201~rm -rf /\r", 0); err != nil {
		t.Fatalf("Insert = %v, want nil", err)
	}
	if got := term.written("s3"); got != "\x1b[200~a[201~rm -rf /\x1b[201~" {
		t.Errorf("written = %q, want the control characters stripped", got)
	}
}

func TestInsertRefusesWhatItCannotPaste(t *testing.T) {
	svc := newRelay(editorWorkspace("/src/lich", "/src/lich-wt"), newFakeTerminal("s3"), nil)

	if _, err := svc.Insert("", "solo", "", " \n\x1b ", 0); err == nil {
		t.Error("an empty text was accepted")
	}
	_, err := svc.Insert("", "solo", "", strings.Repeat("x", insertLimit+1), 0)
	if err == nil || !strings.Contains(err.Error(), "limit") {
		t.Errorf("Insert over the limit = %v, want an error naming it", err)
	}
	if _, err := svc.Insert("", "", "", "x", 0); err == nil {
		t.Error("a call naming neither session nor project was accepted")
	}
}

func TestInsertFindsTheSessionByProjectName(t *testing.T) {
	term := newFakeTerminal("s3")
	svc := newRelay(editorWorkspace("/src/lich", "/src/lich-wt"), term, nil)

	got, err := svc.Insert("", "", "revu", "x", 0)
	if err != nil || got.ID != "s3" {
		t.Errorf("Insert = %+v, %v, want the only session of revu", got, err)
	}
}

func TestInsertByDirectoryPrefersTheSessionRunningThere(t *testing.T) {
	root := t.TempDir()
	worktree := filepath.Join(root, "wt")
	if err := os.Mkdir(worktree, 0o755); err != nil {
		t.Fatal(err)
	}
	term := newFakeTerminal("s1", "s2")
	svc := newRelay(editorWorkspace(root, worktree), term, nil)

	got, err := svc.Insert("", "", worktree, "x", 0)
	if err != nil || got.ID != "s2" {
		t.Errorf("Insert at the worktree = %+v, %v, want the worktree's session", got, err)
	}
	// The project root has a session of its own, so it wins over the worktree's.
	got, err = svc.Insert("", "", root+string(filepath.Separator), "x", 0)
	if err != nil {
		t.Fatalf("Insert at the root = %v, want nil", err)
	}
	if got.ID != "s1" {
		t.Errorf("Insert at the root went to %q, want the session running in it", got.ID)
	}
}

func TestInsertNamesEveryCandidateWhenTheDirectoryIsAmbiguous(t *testing.T) {
	root := t.TempDir()
	work := fakeSessions{projects: []store.Project{
		{ID: "p1", Name: "lich", Path: root, Sessions: []store.Session{
			{ID: "s1", Label: "first"}, {ID: "s2", Label: "second"},
		}},
	}}
	svc := newRelay(work, newFakeTerminal("s1", "s2"), nil)

	_, err := svc.Insert("", "", root, "x", 0)
	if err == nil || !strings.Contains(err.Error(), "first") || !strings.Contains(err.Error(), "second") {
		t.Errorf("Insert = %v, want an error naming both sessions", err)
	}
}

func TestInsertFallsBackToTheProjectsSessionsWhenNoneRunInTheDirectory(t *testing.T) {
	root := t.TempDir()
	work := fakeSessions{projects: []store.Project{
		{ID: "p1", Name: "lich", Path: root, Sessions: []store.Session{
			{ID: "s1", Label: "feature", Path: filepath.Join(root, "wt")},
		}},
	}}
	svc := newRelay(work, newFakeTerminal("s1"), nil)

	got, err := svc.Insert("", "", root, "x", 0)
	if err != nil || got.ID != "s1" {
		t.Errorf("Insert = %+v, %v, want the project's only session", got, err)
	}
}

func TestInsertFindsNothingInAnUnknownProject(t *testing.T) {
	svc := newRelay(editorWorkspace("/src/lich", "/src/lich-wt"), newFakeTerminal("s1"), nil)

	for _, project := range []string{"nope", "/not/a/project"} {
		if _, err := svc.Insert("", "", project, "x", 0); err == nil {
			t.Errorf("Insert into %q succeeded", project)
		}
	}
	// A session that is not running has nothing to paste into.
	if _, err := svc.Insert("", "", "revu", "x", 0); err == nil {
		t.Error("Insert into a project whose only session is stopped succeeded")
	}
}

func TestInsertGoesInBesideTheUsersDraft(t *testing.T) {
	term := newFakeTerminal("s3")
	term.typeAt("s3", true)
	svc := newRelay(editorWorkspace("/src/lich", "/src/lich-wt"), term, nil)

	if _, err := svc.Insert("", "solo", "", "x", 0); err != nil {
		t.Errorf("Insert = %v, want the text added to the draft", err)
	}
}

func TestInsertRefusesASessionBlockedOnAPermission(t *testing.T) {
	term := newFakeTerminal("s3")
	svc := newRelay(editorWorkspace("/src/lich", "/src/lich-wt"), term, nil)
	svc.reported["s3"] = stateWaiting

	_, err := svc.Insert("", "solo", "", "x", 0)
	if err == nil || !strings.Contains(err.Error(), "permission") {
		t.Errorf("Insert = %v, want a refusal naming the permission prompt", err)
	}
	if got := term.written("s3"); got != "" {
		t.Errorf("written = %q, want nothing typed at the dialog", got)
	}
}

func TestInsertGivesUpOnASessionThatNeverReachesItsPrompt(t *testing.T) {
	term := newFakeTerminal("s3")
	term.setUp("s3", true)
	svc := newRelay(editorWorkspace("/src/lich", "/src/lich-wt"), term, nil)

	_, err := svc.insert("", "solo", "", "x", time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "not at a prompt") {
		t.Errorf("insert = %v, want the wait to run out", err)
	}
	if got := term.written("s3"); got != "" {
		t.Errorf("written = %q, want nothing typed into the setup script", got)
	}
}

func TestInsertSurfacesAWriteTheTerminalRefuses(t *testing.T) {
	term := newFakeTerminal("s3")
	term.writeErr = os.ErrClosed
	svc := newRelay(editorWorkspace("/src/lich", "/src/lich-wt"), term, nil)

	if _, err := svc.Insert("", "solo", "", "x", 0); err == nil {
		t.Error("a refused write was reported as inserted")
	}
}

func TestPeersCarryTheirDirectories(t *testing.T) {
	svc := newRelay(editorWorkspace("/src/lich", "/src/lich-wt"), newFakeTerminal("s1", "s2"), nil)

	peers, err := svc.Peers("")
	if err != nil || len(peers) != 2 {
		t.Fatalf("Peers = %v, %v", peers, err)
	}
	if peers[0].Path != "/src/lich" || peers[0].ProjectPath != "/src/lich" {
		t.Errorf("a root session = %+v, want the project's directory for both", peers[0])
	}
	if peers[1].Path != "/src/lich-wt" || peers[1].ProjectPath != "/src/lich" {
		t.Errorf("a worktree session = %+v, want its own directory and the project's", peers[1])
	}
}

func TestSendNarrowsByProjectDirectory(t *testing.T) {
	root := t.TempDir()
	work := fakeSessions{projects: []store.Project{
		{ID: "p1", Name: "a", Path: root, Sessions: []store.Session{{ID: "s1", Label: "api"}}},
		{ID: "p2", Name: "b", Path: filepath.Join(root, "b"), Sessions: []store.Session{{ID: "s2", Label: "api"}}},
	}}
	svc := newRelay(work, newFakeTerminal("s1", "s2"), nil)

	dest, err := svc.resolve("", "api", filepath.Join(root, "b"))
	if err != nil || dest.ID != "s2" {
		t.Errorf("resolve by directory = %+v, %v, want the session of the project rooted there", dest, err)
	}
}

func TestSamePath(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	linked := os.Symlink(dir, link) == nil

	cases := []struct {
		name string
		a, b string
		want bool
	}{
		{"identical", dir, dir, true},
		{"trailing separator", dir + string(filepath.Separator), dir, true},
		{"different", dir, filepath.Dir(dir), false},
		{"empty", "", dir, false},
		{"relative", "relative/dir", "relative/dir", false},
		{"missing on both sides", filepath.Join(dir, "x"), filepath.Join(dir, "y"), false},
	}
	if linked {
		cases = append(cases, struct {
			name string
			a, b string
			want bool
		}{"symlink", link, dir, true})
	}
	for _, c := range cases {
		if got := samePath(c.a, c.b); got != c.want {
			t.Errorf("%s: samePath(%q, %q) = %v, want %v", c.name, c.a, c.b, got, c.want)
		}
	}
}

func TestExpandHome(t *testing.T) {
	t.Setenv("HOME", "/home/me")
	t.Setenv("USERPROFILE", `C:\Users\me`)
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory here")
	}
	if got := expandHome("~"); got != home {
		t.Errorf("expandHome(~) = %q, want %q", got, home)
	}
	if got := expandHome("~/src"); got != filepath.Join(home, "src") {
		t.Errorf("expandHome(~/src) = %q", got)
	}
	if got := expandHome("/abs"); got != "/abs" {
		t.Errorf("expandHome(/abs) = %q, want it untouched", got)
	}
}
