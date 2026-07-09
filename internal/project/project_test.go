package project

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

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
