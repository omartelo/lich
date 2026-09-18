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

// TestCarryUncommittedSkipsNonRegular proves a symlink git listed as untracked
// is stepped over rather than dereferenced or reported: the rest of the carry
// still lands.
func TestCarryUncommittedSkipsNonRegular(t *testing.T) {
	repo, git := initRepo(t)
	if err := os.Symlink("a.txt", filepath.Join(repo, "link.txt")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	write(t, repo, "real.txt", "carried\n")

	dst := filepath.Join(t.TempDir(), "fork")
	git("worktree", "add", "-b", "fork", dst, "main")

	if err := New(nil).CarryUncommitted(repo, dst); err != nil {
		t.Fatalf("CarryUncommitted: %v", err)
	}
	if got := read(t, dst, "real.txt"); got != "carried\n" {
		t.Errorf("real.txt = %q, want the carried content", got)
	}
	if _, err := os.Lstat(filepath.Join(dst, "link.txt")); !os.IsNotExist(err) {
		t.Errorf("the symlink was carried (%v)", err)
	}
}

// TestCarryFileVanished proves a file that is gone by the time it is copied —
// the listing and the copy are two calls, and a build in the source checkout
// deletes its own scratch between them — is not a failure.
func TestCarryFileVanished(t *testing.T) {
	dir := t.TempDir()
	if err := carryFile(filepath.Join(dir, "gone.txt"), filepath.Join(dir, "copy.txt")); err != nil {
		t.Fatalf("carryFile on a missing source: %v", err)
	}
}

// TestCarryFileReportsRealFailure proves the two skips are the only ones, and
// that they are decided on the source rather than read out of the error: a
// destination whose parent is a file answers ERROR_PATH_NOT_FOUND on Windows,
// which Go maps onto fs.ErrNotExist — the same error a vanished source gives.
// Caught by the Windows runner after passing here (#616).
func TestCarryFileReportsRealFailure(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "from.txt", "data\n")
	// The destination's parent is a file, so MkdirAll cannot make the directory.
	write(t, dir, "blocked", "")
	if err := carryFile(filepath.Join(dir, "from.txt"), filepath.Join(dir, "blocked", "to.txt")); err == nil {
		t.Fatal("carryFile reported success on a destination it cannot create")
	}
}
