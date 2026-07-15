package project

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDiscardFileTracked proves a modified tracked file is restored to its
// HEAD content, staged or not.
func TestDiscardFileTracked(t *testing.T) {
	repo, git := initRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("uno\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "a.txt")

	if err := New(nil).DiscardFile(repo, "a.txt"); err != nil {
		t.Fatalf("DiscardFile: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(repo, "a.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "one\n" {
		t.Errorf("content = %q, want %q", got, "one\n")
	}
	if out := git("status", "--porcelain"); out != "" {
		t.Errorf("status not clean after discard: %q", out)
	}
}

// TestDiscardFileNew proves untracked and staged-new files are removed from
// disk and index.
func TestDiscardFileNew(t *testing.T) {
	repo, git := initRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "staged.txt"), []byte("s\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "staged.txt")
	if err := os.WriteFile(filepath.Join(repo, "loose.txt"), []byte("l\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, rel := range []string{"staged.txt", "loose.txt"} {
		if err := New(nil).DiscardFile(repo, rel); err != nil {
			t.Fatalf("DiscardFile(%s): %v", rel, err)
		}
		if _, err := os.Stat(filepath.Join(repo, rel)); !os.IsNotExist(err) {
			t.Errorf("%s still on disk after discard", rel)
		}
	}
	if out := git("status", "--porcelain"); out != "" {
		t.Errorf("status not clean after discard: %q", out)
	}
}

// TestDiscardFileRejectsEscape proves traversal and absolute paths never reach
// the filesystem.
func TestDiscardFileRejectsEscape(t *testing.T) {
	repo, _ := initRepo(t)
	for _, rel := range []string{"../outside.txt", "/etc/passwd", "a/../../b"} {
		if err := New(nil).DiscardFile(repo, rel); err == nil {
			t.Errorf("DiscardFile(%q): want error, got nil", rel)
		}
	}
}
