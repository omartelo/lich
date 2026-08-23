package project

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type pickerStub struct {
	dirPath   string
	dirError  error
	dirTitle  string
	filePath  string
	fileError error
	fileTitle string
	saveName  string
}

func (p *pickerStub) PickDirectory(title string) (string, error) {
	p.dirTitle = title
	return p.dirPath, p.dirError
}

func (p *pickerStub) PickFile(title string) (string, error) {
	p.fileTitle = title
	return p.filePath, p.fileError
}

func (p *pickerStub) PickSaveFile(title, defaultName string) (string, error) {
	p.fileTitle = title
	p.saveName = defaultName
	return p.filePath, p.fileError
}

func TestPickFileUsesRequestedTitle(t *testing.T) {
	picker := &pickerStub{filePath: "/tmp/theme.json"}
	path, err := New(picker).PickFile("Import Theme")
	if err != nil || path != picker.filePath || picker.fileTitle != "Import Theme" {
		t.Fatalf("PickFile = %q, %v; title = %q", path, err, picker.fileTitle)
	}

	picker.fileError = errors.New("picker failed")
	if _, err := New(picker).PickFile("Import Theme"); err == nil {
		t.Fatal("PickFile error was swallowed")
	}
}

func TestPickSaveFileSeedsTheDefaultName(t *testing.T) {
	picker := &pickerStub{filePath: "/tmp/lich-theme-template.json"}
	path, err := New(picker).PickSaveFile("Save Theme Template", "lich-theme-template.json")
	if err != nil || path != picker.filePath {
		t.Fatalf("PickSaveFile = %q, %v", path, err)
	}
	if picker.fileTitle != "Save Theme Template" || picker.saveName != "lich-theme-template.json" {
		t.Fatalf("dialog seeded with title %q, name %q", picker.fileTitle, picker.saveName)
	}

	picker.fileError = errors.New("picker failed")
	if _, err := New(picker).PickSaveFile("Save Theme Template", "x.json"); err == nil {
		t.Fatal("PickSaveFile error was swallowed")
	}
}

// TestOpen covers the three answers the dialog can give. The cancelled one is
// the contract that matters: the frontend reads Project-or-null, so a cancel
// has to come back as nil with no error — an error there would put a failure
// dialog on a user who simply closed the picker.
func TestOpen(t *testing.T) {
	dir := t.TempDir()
	picker := &pickerStub{dirPath: dir}
	p, err := New(picker).Open()
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	if p == nil || p.Path != dir || p.ID != projectID(dir) {
		t.Fatalf("Open() = %+v, want the project for %q", p, dir)
	}
	if picker.dirTitle != "Open Project" {
		t.Errorf("dialog title = %q, want Open Project", picker.dirTitle)
	}

	cancelled := &pickerStub{dirPath: ""}
	if p, err := New(cancelled).Open(); p != nil || err != nil {
		t.Errorf("Open(cancelled) = %+v, %v; want nil, nil", p, err)
	}

	failed := &pickerStub{dirError: errors.New("zenity missing")}
	p, err = New(failed).Open()
	if err == nil {
		t.Fatal("Open() swallowed the picker error")
	}
	if p != nil {
		t.Errorf("Open(failed) = %+v, want nil", p)
	}
}

// TestProjectID proves the ID is deterministic per path and differs across
// paths.
func TestProjectID(t *testing.T) {
	a := projectID("/tmp/alpha")
	again := projectID("/tmp/alpha")
	b := projectID("/tmp/beta")

	if a != again {
		t.Errorf("projectID not deterministic: %q != %q", a, again)
	}
	if a == b {
		t.Errorf("projectID collided for different paths: %q", a)
	}
	if len(a) != projectIDBytes*2 {
		t.Errorf("len(projectID) = %d, want %d", len(a), projectIDBytes*2)
	}
}

// TestNewProject proves the path→Project mapping the dialog feeds into: Name is
// the base directory, Path is verbatim, ID matches projectID.
func TestNewProject(t *testing.T) {
	p := newProject("/tmp/some/alpha")
	if p.Name != "alpha" {
		t.Errorf("Name = %q, want alpha", p.Name)
	}
	if p.Path != "/tmp/some/alpha" {
		t.Errorf("Path = %q, want /tmp/some/alpha", p.Path)
	}
	if p.ID != projectID("/tmp/some/alpha") {
		t.Errorf("ID = %q, want %q", p.ID, projectID("/tmp/some/alpha"))
	}
}

func TestHome(t *testing.T) {
	dir := t.TempDir()
	// os.UserHomeDir reads HOME on Unix and USERPROFILE on Windows.
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)

	p, err := New(nil).Home()
	if err != nil {
		t.Fatalf("Home() error: %v", err)
	}
	if p.Path != dir {
		t.Errorf("Path = %q, want %q", p.Path, dir)
	}
	if p.ID != projectID(dir) {
		t.Errorf("ID = %q, want the stable id for %q", p.ID, dir)
	}
}

// TestBranch proves Branch reads the checked-out branch of a git work tree and
// returns "" for a directory that is not a repository.
func TestBranch(t *testing.T) {
	repo := t.TempDir()
	if out, err := exec.Command("git", "init", "-b", "trunk", repo).CombinedOutput(); err != nil {
		t.Skipf("git init unavailable: %v (%s)", err, out)
	}

	svc := New(nil)
	if got := svc.Branch(repo); got != "trunk" {
		t.Errorf("Branch(repo) = %q, want trunk", got)
	}
	if got := svc.Branch(t.TempDir()); got != "" {
		t.Errorf("Branch(non-repo) = %q, want empty", got)
	}
}

// TestBranchesOf proves the batch answers one entry per checkout that names a
// branch and leaves the rest out entirely — the history rows read "no branch"
// from a missing key, so a path mapped to "" would draw an empty badge instead
// of none at all.
func TestBranchesOf(t *testing.T) {
	repo := t.TempDir()
	if out, err := exec.Command("git", "init", "-b", "trunk", repo).CombinedOutput(); err != nil {
		t.Skipf("git init unavailable: %v (%s)", err, out)
	}
	notARepo := t.TempDir()
	gone := filepath.Join(t.TempDir(), "removed-behind-our-back")

	got := New(nil).BranchesOf([]string{repo, notARepo, gone})
	if len(got) != 1 || got[repo] != "trunk" {
		t.Errorf("BranchesOf = %v, want only the real checkout, on trunk", got)
	}
}

// TestBranchesOfNothing keeps the empty call cheap and non-nil: the window calls
// it on every palette opening, including the one where no session has ever been
// closed.
func TestBranchesOfNothing(t *testing.T) {
	if got := New(nil).BranchesOf(nil); got == nil || len(got) != 0 {
		t.Errorf("BranchesOf(nil) = %v, want an empty map", got)
	}
}

// TestParsePullRequest proves the gh output decoder: a real PR yields its number
// and URL, while malformed JSON, an empty object, and a PR missing its number or
// URL all collapse to nil so the badge hides instead of showing garbage.
func TestParsePullRequest(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want *PullRequest
	}{
		{"open", `{"number":7,"url":"https://github.com/o/r/pull/7","state":"OPEN"}`, &PullRequest{Number: 7, URL: "https://github.com/o/r/pull/7", State: "OPEN"}},
		{"merged", `{"number":7,"url":"https://github.com/o/r/pull/7","state":"MERGED"}`, nil},
		{"closed", `{"number":7,"url":"https://github.com/o/r/pull/7","state":"CLOSED"}`, nil},
		{"missing state", `{"number":7,"url":"https://github.com/o/r/pull/7"}`, nil},
		{"empty object", `{}`, nil},
		{"zero number", `{"number":0,"url":"https://github.com/o/r/pull/0","state":"OPEN"}`, nil},
		{"missing url", `{"number":7,"state":"OPEN"}`, nil},
		{"malformed", `not json`, nil},
		{"empty bytes", ``, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parsePullRequest([]byte(tt.out))
			switch {
			case tt.want == nil && got != nil:
				t.Errorf("parsePullRequest(%q) = %+v, want nil", tt.out, got)
			case tt.want != nil && (got == nil || *got != *tt.want):
				t.Errorf("parsePullRequest(%q) = %+v, want %+v", tt.out, got, tt.want)
			}
		})
	}
}

// TestDiff proves Diff counts dirty files and added/deleted lines against HEAD,
// and returns the zero value for clean repositories and non-repositories.
func TestDiff(t *testing.T) {
	repo := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
	}
	if out, err := exec.Command("git", "init", repo).CombinedOutput(); err != nil {
		t.Skipf("git init unavailable: %v (%s)", err, out)
	}

	svc := New(nil)
	file := filepath.Join(repo, "a.txt")
	if err := os.WriteFile(file, []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "a.txt")
	git("commit", "-m", "init")

	clean := svc.Diff(repo)
	if clean.Files != 0 || clean.Added != 0 || clean.Deleted != 0 {
		t.Errorf("Diff(clean repo) = %+v, want no changes", clean)
	}
	// Head names the commit the counts sit on — the signal the frontend watches
	// to notice a commit, so a clean tree still has to report it.
	if len(clean.Head) < 7 {
		t.Errorf("Diff(clean repo).Head = %q, want a commit sha", clean.Head)
	}

	if err := os.WriteFile(file, []byte("one\nchanged\nadded\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := svc.Diff(repo)
	if got.Files != 1 || got.Added != 2 || got.Deleted != 1 {
		t.Errorf("Diff(edited file) = %+v, want {Files:1 Added:2 Deleted:1}", got)
	}

	// Untracked lines count as additions: 3 lines, the last without a trailing
	// newline. On top of the tracked edit this makes Added 2+3.
	if err := os.WriteFile(filepath.Join(repo, "new.txt"), []byte("x\ny\nz"), 0o644); err != nil {
		t.Fatal(err)
	}
	got = svc.Diff(repo)
	if got.Files != 2 || got.Added != 5 {
		t.Errorf("Diff(untracked added) = %+v, want {Files:2 Added:5 Deleted:1}", got)
	}

	// Binary untracked files add no lines.
	if err := os.WriteFile(filepath.Join(repo, "bin.dat"), []byte("a\x00b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := svc.Diff(repo); got.Added != 5 {
		t.Errorf("Diff(untracked binary).Added = %d, want 5", got.Added)
	}

	// Untracked files inside a new directory count one by one: git's default
	// porcelain collapses them into a single "?? pkg/" entry, which would report
	// a whole new package as one changed file.
	if err := os.MkdirAll(filepath.Join(repo, "pkg", "deep"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"one.txt", "two.txt"} {
		if err := os.WriteFile(filepath.Join(repo, "pkg", "deep", name), []byte("l\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if got := svc.Diff(repo); got.Files != 5 || got.Added != 7 {
		t.Errorf("Diff(new directory) = %+v, want {Files:5 Added:7 Deleted:1}", got)
	}

	if got := svc.Diff(t.TempDir()); got != (DiffStats{}) {
		t.Errorf("Diff(non-repo) = %+v, want zero", got)
	}
}

// TestDiffCarriesTheBranch pins the field the status badge now reads its branch
// from. It rides Diff rather than a call of its own — that is the subprocess
// the collapse saves — so the detached case has to hold here too, not only in
// Branch.
func TestDiffCarriesTheBranch(t *testing.T) {
	repo, git := initRepo(t)

	if got := New(nil).Diff(repo); got.Branch != "main" {
		t.Errorf("Diff(repo).Branch = %q, want main", got.Branch)
	}
	git("checkout", "--detach", "HEAD")
	if got := New(nil).Diff(repo); got.Branch != "" {
		t.Errorf("Diff(detached).Branch = %q, want empty", got.Branch)
	}
}

// TestDiffNoCommits covers a freshly `git init`'d repo with no HEAD: numstat
// against HEAD fails, and the untracked additions must still be counted rather
// than the whole stat collapsing to +0 -0.
func TestDiffNoCommits(t *testing.T) {
	repo := t.TempDir()
	if out, err := exec.Command("git", "init", repo).CombinedOutput(); err != nil {
		t.Skipf("git init unavailable: %v (%s)", err, out)
	}
	if err := os.WriteFile(filepath.Join(repo, "new.txt"), []byte("x\ny\nz\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := New(nil).Diff(repo)
	if got.Files != 1 || got.Added != 3 || got.Deleted != 0 {
		t.Errorf("Diff(no-commit repo) = %+v, want {Files:1 Added:3 Deleted:0}", got)
	}
	// No HEAD to name yet; the field stays empty rather than carrying the empty
	// tree hash the numstat falls back to.
	if got.Head != "" {
		t.Errorf("Diff(no-commit repo).Head = %q, want empty", got.Head)
	}
}

// TestDiffSpendsNoNumstatOnACleanTree pins the call budget itself, not only the
// numbers: a checkout with nothing dirty must answer from the status read
// alone. The counts are asserted alongside, because a skipped call that also
// dropped a number would be a regression wearing the optimisation's clothes.
//
// The children are counted through git's own GIT_TRACE, so nothing in the
// package has to grow a seam to be observed. The dirty cases assert the *same*
// trace line is present — without them a git that stopped writing the trace, or
// a path it refused, would turn the clean assertion into one that can no longer
// fail.
func TestDiffSpendsNoNumstatOnACleanTree(t *testing.T) {
	repo, git := initRepo(t)
	trace := filepath.Join(t.TempDir(), "git-trace.log")
	t.Setenv("GIT_TRACE", trace)

	numstats := func() int {
		t.Helper()
		// Absent until the first git child runs, which is the zero the clean
		// case expects to read.
		out, err := os.ReadFile(trace)
		if err != nil && !os.IsNotExist(err) {
			t.Fatalf("read %s: %v", trace, err)
		}
		return strings.Count(string(out), "diff --numstat")
	}

	svc := New(nil)
	if got := svc.Diff(repo); got.Added != 0 || got.Deleted != 0 {
		t.Errorf("Diff(clean) = %+v, want no lines", got)
	}
	if spent := numstats(); spent != 0 {
		t.Errorf("Diff(clean) spent %d numstat calls, want 0", spent)
	}

	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := svc.Diff(repo); got.Files != 1 || got.Added != 1 || got.Deleted != 0 {
		t.Errorf("Diff(edited) = %+v, want {Files:1 Added:1 Deleted:0}", got)
	}
	if spent := numstats(); spent != 1 {
		t.Errorf("Diff(edited) spent %d numstat calls, want 1", spent)
	}

	// Untracked-only is the case the skip must not swallow: the tree is dirty,
	// so the call is spent, and git legitimately answers with no lines — the
	// additions come from reading the file instead.
	git("checkout", "--", "a.txt")
	if err := os.WriteFile(filepath.Join(repo, "new.txt"), []byte("x\ny\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := svc.Diff(repo); got.Files != 1 || got.Added != 2 || got.Deleted != 0 {
		t.Errorf("Diff(untracked only) = %+v, want {Files:1 Added:2 Deleted:0}", got)
	}
	if spent := numstats(); spent != 2 {
		t.Errorf("Diff(untracked only) spent %d numstat calls in total, want 2", spent)
	}
}

// sparseTextFile writes a text prefix and stretches the file to size with a
// hole, so a file past the read cap costs neither disk nor a 10MiB write. The
// prefix has to cover the whole binary sniff window: a shorter one leaves NULs
// inside it, and the file would be refused as binary whether or not the cap
// exists — a test that could not tell the two apart.
func sparseTextFile(t *testing.T, name string, size int64) {
	t.Helper()
	f, err := os.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(strings.Repeat("text\n", binarySniffBytes)); err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(size); err != nil {
		t.Fatal(err)
	}
}

// TestCountFileLinesRefusesAFileOverTheCap pins maxTextFileBytes at 10MiB: an
// untracked file past it adds no lines to the diff counters rather than being
// read whole into memory.
func TestCountFileLinesRefusesAFileOverTheCap(t *testing.T) {
	dir := t.TempDir()
	small := filepath.Join(dir, "small.txt")
	if err := os.WriteFile(small, []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := countFileLines(small); got != 2 {
		t.Errorf("countFileLines(small) = %d, want 2", got)
	}

	big := filepath.Join(dir, "big.txt")
	sparseTextFile(t, big, 10<<20+1)
	if got := countFileLines(big); got != 0 {
		t.Errorf("countFileLines(over 10MiB) = %d, want 0", got)
	}
}

// TestExistsAcceptsOnlyDirectories: a project path that now names a file is as
// unopenable as one that names nothing, so both have to read as gone.
func TestExistsAcceptsOnlyDirectories(t *testing.T) {
	svc := New(nil)
	dir := t.TempDir()
	file := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	if !svc.Exists(dir) {
		t.Errorf("Exists(%q) = false, want true", dir)
	}
	if svc.Exists(file) {
		t.Errorf("Exists(file) = true, want false")
	}
	if svc.Exists(filepath.Join(dir, "gone")) {
		t.Errorf("Exists(missing) = true, want false")
	}
}

// TestMissingNamesOnlyTheGoneDirectories: the answer is what the lists mark, so
// a path that is still there must never appear in it.
func TestMissingNamesOnlyTheGoneDirectories(t *testing.T) {
	svc := New(nil)
	dir := t.TempDir()
	gone := filepath.Join(dir, "gone")

	missing := svc.Missing([]string{dir, gone})
	if len(missing) != 1 || missing[0] != gone {
		t.Fatalf("Missing = %v, want [%q]", missing, gone)
	}
	if got := svc.Missing(nil); len(got) != 0 {
		t.Errorf("Missing(nil) = %v, want empty", got)
	}
}

// TestRelocateKeepsTheProjectIdentity pins the whole point of the call: the id
// comes from the caller, not from the new path. Deriving it the way Open does
// would hand back a project the store has never seen, and the sessions and the
// worktree directory of the one that moved would stay with the dead id.
func TestRelocateKeepsTheProjectIdentity(t *testing.T) {
	moved := filepath.Join(t.TempDir(), "renamed")
	picker := &pickerStub{dirPath: moved}
	svc := New(picker)

	project, err := svc.Relocate("stored-id")
	if err != nil {
		t.Fatalf("Relocate: %v", err)
	}
	if project == nil {
		t.Fatal("Relocate = nil, want the repointed project")
	}
	if project.ID != "stored-id" {
		t.Errorf("id = %q, want the stored one", project.ID)
	}
	if project.Path != moved || project.Name != "renamed" {
		t.Errorf("project = %+v, want path %q named after it", project, moved)
	}
	if picker.dirTitle != "Relocate Project" {
		t.Errorf("dialog title = %q", picker.dirTitle)
	}
}

// TestRelocateRefusesADirectoryAnotherProjectHolds: the path comes from a
// dialog, so it is user input and the workspace is what validates it. Two rows
// on one directory under different ids is a workspace no path-addressed lookup
// can read — and no undo puts the sessions back afterwards.
func TestRelocateRefusesADirectoryAnotherProjectHolds(t *testing.T) {
	taken := filepath.Join(t.TempDir(), "taken")
	svc := New(&pickerStub{dirPath: taken})
	svc.SetProjects(func(path string) (string, string) {
		if path == taken {
			return "other-id", "warm-fjord"
		}
		return "", ""
	})

	project, err := svc.Relocate("stored-id")
	if project != nil {
		t.Fatalf("Relocate = %+v, want nothing relocated", project)
	}
	if err == nil || !strings.Contains(err.Error(), "warm-fjord") {
		t.Fatalf("Relocate error = %v, want the project holding it named", err)
	}
}

// TestRelocateAcceptsTheProjectsOwnDirectory: the guard is about a *second*
// project on a path. A row pointed at the directory it is already stored with
// is the relocation being repeated, not a collision, and refusing it would
// dead-end the only flow that can fix a project.
func TestRelocateAcceptsTheProjectsOwnDirectory(t *testing.T) {
	own := filepath.Join(t.TempDir(), "own")
	svc := New(&pickerStub{dirPath: own})
	svc.SetProjects(func(string) (string, string) { return "stored-id", "own" })

	project, err := svc.Relocate("stored-id")
	if err != nil || project == nil || project.Path != own {
		t.Fatalf("Relocate = %+v, %v; want the project repointed at %q", project, err, own)
	}
}

// TestRelocateCancelledChangesNothing: a cancelled dialog must not read as a
// chosen directory — the caller writes whatever comes back over the stored row.
func TestRelocateCancelledChangesNothing(t *testing.T) {
	project, err := New(&pickerStub{}).Relocate("stored-id")
	if err != nil || project != nil {
		t.Fatalf("Relocate(cancelled) = %+v, %v; want nil, nil", project, err)
	}

	if _, err := New(&pickerStub{dirError: errors.New("picker failed")}).Relocate("stored-id"); err == nil {
		t.Fatal("Relocate error was swallowed")
	}
}
