package project

import (
	"os"
	"path/filepath"
	"testing"
)

// TestCarryUncommitted proves the whole shape of a dirty checkout reaches the
// fork: a modification, a deletion, a file staged but never committed, an
// untracked file in a directory that does not exist in dst yet, and a binary
// change. What git ignores stays behind, and everything arrives unstaged.
func TestCarryUncommitted(t *testing.T) {
	repo, git := initRepo(t)
	write(t, repo, "mod.txt", "one\n")
	write(t, repo, "del.txt", "gone\n")
	write(t, repo, "bin.dat", "\x00\x01\x02")
	write(t, repo, ".gitignore", "ignored.log\n")
	git("add", "-A")
	git("commit", "-m", "second")

	write(t, repo, "mod.txt", "one\ntwo\n")
	if err := os.Remove(filepath.Join(repo, "del.txt")); err != nil {
		t.Fatal(err)
	}
	write(t, repo, "bin.dat", "\x00\xff\x02")
	write(t, repo, "staged.txt", "staged\n")
	git("add", "staged.txt")
	write(t, repo, "new/deep.txt", "untracked\n")
	write(t, repo, "ignored.log", "noise\n")

	dst := filepath.Join(t.TempDir(), "fork")
	git("worktree", "add", "-b", "fork", dst, "main")

	if err := New(nil).CarryUncommitted(repo, dst); err != nil {
		t.Fatalf("CarryUncommitted: %v", err)
	}

	for name, want := range map[string]string{
		"mod.txt":      "one\ntwo\n",
		"bin.dat":      "\x00\xff\x02",
		"staged.txt":   "staged\n",
		"new/deep.txt": "untracked\n",
	} {
		if got := read(t, dst, name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	if _, err := os.Stat(filepath.Join(dst, "del.txt")); !os.IsNotExist(err) {
		t.Errorf("del.txt still in the fork (%v)", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "ignored.log")); !os.IsNotExist(err) {
		t.Errorf("ignored.log was carried (%v)", err)
	}
	if staged := gitIn(t, dst)("diff", "--cached", "--name-only"); staged != "" {
		t.Errorf("the fork has staged files: %q", staged)
	}
}

// TestCarryUncommittedClean proves a clean source is a no-op rather than an
// error — the fork of a committed checkout goes through the same call.
func TestCarryUncommittedClean(t *testing.T) {
	repo, git := initRepo(t)
	dst := filepath.Join(t.TempDir(), "fork")
	git("worktree", "add", "-b", "fork", dst, "main")

	if err := New(nil).CarryUncommitted(repo, dst); err != nil {
		t.Fatalf("CarryUncommitted: %v", err)
	}
	if status := gitIn(t, dst)("status", "--porcelain"); status != "" {
		t.Errorf("fork is dirty:\n%s", status)
	}
}

// TestCarryUncommittedNotRepo proves a source git cannot read is reported
// rather than silently carrying nothing.
func TestCarryUncommittedNotRepo(t *testing.T) {
	if err := New(nil).CarryUncommitted(t.TempDir(), t.TempDir()); err == nil {
		t.Fatal("CarryUncommitted accepted a directory that is not a checkout")
	}
}

func read(t *testing.T, dir, name string) string {
	t.Helper()
	got, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	return string(got)
}
