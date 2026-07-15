package project

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestDiffText proves tracked edits (staged and unstaged) and untracked files
// all land in one unified diff.
func TestDiffText(t *testing.T) {
	repo, git := initRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("uno\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "staged.txt"), []byte("s\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "staged.txt")
	if err := os.WriteFile(filepath.Join(repo, "new.txt"), []byte("x\ny\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := New(nil).DiffText(repo)
	if err != nil {
		t.Fatalf("DiffText: %v", err)
	}
	for _, want := range []string{
		"diff --git a/a.txt b/a.txt", "-one", "+uno",
		"+++ b/staged.txt", "+s",
		"+++ b/new.txt", "new file mode", "+x", "+y",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("diff missing %q:\n%s", want, out)
		}
	}
}

// TestDiffTextNoHead proves a repository without commits diffs staged and
// untracked files against the empty tree instead of erroring on HEAD.
func TestDiffTextNoHead(t *testing.T) {
	repo := t.TempDir()
	if out, err := exec.Command("git", "init", "-b", "main", repo).CombinedOutput(); err != nil {
		t.Skipf("git init unavailable: %v (%s)", err, out)
	}
	if err := os.WriteFile(filepath.Join(repo, "staged.txt"), []byte("s\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", repo, "add", "staged.txt").CombinedOutput(); err != nil {
		t.Fatalf("git add: %v (%s)", err, out)
	}
	if err := os.WriteFile(filepath.Join(repo, "loose.txt"), []byte("l\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := New(nil).DiffText(repo)
	if err != nil {
		t.Fatalf("DiffText: %v", err)
	}
	for _, want := range []string{"+++ b/staged.txt", "+++ b/loose.txt"} {
		if !strings.Contains(out, want) {
			t.Errorf("diff missing %q:\n%s", want, out)
		}
	}
}

// TestDiffTextClean proves a clean repository yields an empty diff, and a
// non-repository path an error.
func TestDiffTextClean(t *testing.T) {
	repo, _ := initRepo(t)
	out, err := New(nil).DiffText(repo)
	if err != nil {
		t.Fatalf("DiffText: %v", err)
	}
	if out != "" {
		t.Errorf("clean repo diff = %q, want empty", out)
	}

	if _, err := New(nil).DiffText(t.TempDir()); err == nil {
		t.Error("non-repo path: want error, got nil")
	}
}

// TestDiffTextBinaryUntracked proves an untracked binary shows up as git's
// "Binary files ... differ" stanza rather than a textual hunk.
func TestDiffTextBinaryUntracked(t *testing.T) {
	repo, _ := initRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "blob.bin"), []byte{0, 1, 2, 0}, 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := New(nil).DiffText(repo)
	if err != nil {
		t.Fatalf("DiffText: %v", err)
	}
	if !strings.Contains(out, "Binary files") || !strings.Contains(out, "blob.bin") {
		t.Errorf("binary untracked missing from diff:\n%s", out)
	}
}
