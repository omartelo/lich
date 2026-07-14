package project

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

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

// TestBranch proves Branch reads the checked-out branch of a git work tree and
// returns "" for a directory that is not a repository.
func TestBranch(t *testing.T) {
	repo := t.TempDir()
	if out, err := exec.Command("git", "init", "-b", "trunk", repo).CombinedOutput(); err != nil {
		t.Skipf("git init unavailable: %v (%s)", err, out)
	}

	svc := New()
	if got := svc.Branch(repo); got != "trunk" {
		t.Errorf("Branch(repo) = %q, want trunk", got)
	}
	if got := svc.Branch(t.TempDir()); got != "" {
		t.Errorf("Branch(non-repo) = %q, want empty", got)
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

	svc := New()
	file := filepath.Join(repo, "a.txt")
	if err := os.WriteFile(file, []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "a.txt")
	git("commit", "-m", "init")

	if got := svc.Diff(repo); got != (DiffStats{}) {
		t.Errorf("Diff(clean repo) = %+v, want zero", got)
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

	if got := svc.Diff(t.TempDir()); got != (DiffStats{}) {
		t.Errorf("Diff(non-repo) = %+v, want zero", got)
	}
}
