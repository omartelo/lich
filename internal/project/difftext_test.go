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

// TestUntrackedDiffYieldsNothingItCannotRender pins the three ways an untracked
// file produces no hunk instead of an error: it is gone by the time the diff
// runs (ls-files listed it a moment earlier), it is past the read cap, or git
// itself cannot open it — the last one is the "any other failure" branch, which
// has to be told apart from git's exit 1 for "the files differ".
func TestUntrackedDiffYieldsNothingItCannotRender(t *testing.T) {
	repo, _ := initRepo(t)

	if got := untrackedDiff(repo, "vanished.txt"); got != "" {
		t.Errorf("untrackedDiff(vanished) = %q, want empty", got)
	}

	sparseTextFile(t, filepath.Join(repo, "huge.txt"), 10<<20+1)
	// Length, not content: a regression here renders the whole 10MiB file.
	if got := untrackedDiff(repo, "huge.txt"); got != "" {
		t.Errorf("untrackedDiff(over 10MiB) = %d bytes, want empty", len(got))
	}

	unreadable := filepath.Join(repo, "unreadable.txt")
	if err := os.WriteFile(unreadable, []byte("x\ny\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(unreadable, 0o000); err != nil {
		t.Fatal(err)
	}
	if _, err := os.ReadFile(unreadable); err == nil {
		t.Skip("file permissions do not apply here")
	}
	if got := untrackedDiff(repo, "unreadable.txt"); got != "" {
		t.Errorf("untrackedDiff(unreadable) = %q, want empty", got)
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
