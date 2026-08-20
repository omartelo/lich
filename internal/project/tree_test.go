package project

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestTree proves the tree reflects the work tree: tracked files listed
// repo-relative and slash-separated, untracked-but-not-ignored files included,
// .gitignore'd files excluded, all sorted.
func TestTree(t *testing.T) {
	repo, git := initRepo(t)
	mkdir(t, repo, "internal/rpc")
	write(t, repo, "internal/rpc/rpc.go", "package rpc\n")
	write(t, repo, "z.txt", "z\n")
	git("add", ".")
	git("commit", "-m", "add files")

	// Ignored files stay invisible; an untracked file now shows without a commit.
	write(t, repo, ".gitignore", "ignored.txt\n")
	write(t, repo, "ignored.txt", "secret\n")
	write(t, repo, "untracked.txt", "new\n")
	git("add", ".gitignore")
	git("commit", "-m", "gitignore")

	files, err := New(nil).Tree(repo)
	if err != nil {
		t.Fatalf("Tree: %v", err)
	}
	got := strings.Join(files, ",")
	want := ".gitignore,a.txt,internal/rpc/rpc.go,untracked.txt,z.txt"
	if got != want {
		t.Errorf("Tree = %q, want %q", got, want)
	}
}

// TestTreeDropsDeleted proves a tracked file removed from disk (but not yet
// staged) disappears from the tree, so the list is not frozen at HEAD.
func TestTreeDropsDeleted(t *testing.T) {
	repo, _ := initRepo(t)
	if err := os.Remove(filepath.Join(repo, "a.txt")); err != nil {
		t.Fatal(err)
	}
	files, err := New(nil).Tree(repo)
	if err != nil {
		t.Fatalf("Tree: %v", err)
	}
	if slices.Contains(files, "a.txt") {
		t.Errorf("Tree = %v, want a.txt dropped", files)
	}
}

// TestTreeWalksNonRepo proves a plain directory is still browsable: the files
// come back root-relative, slash-separated and sorted, nested folders included
// and .git skipped whole. The contract changed here — a non-repository path used
// to be an error, matching DiffText; browsing files is not a git feature.
func TestTreeWalksNonRepo(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "b.txt", "b\n")
	write(t, dir, "src/main.go", "package main\n")
	write(t, dir, ".git/config", "[core]\n")
	files, err := New(nil).Tree(dir)
	if err != nil {
		t.Fatalf("Tree: %v", err)
	}
	if got, want := strings.Join(files, ","), "b.txt,src/main.go"; got != want {
		t.Errorf("Tree = %q, want %q", got, want)
	}
}

// TestTreeMissingDir proves an absent directory is still an error, so the panel
// says so instead of drawing an empty tree over a folder that is gone.
func TestTreeMissingDir(t *testing.T) {
	if _, err := New(nil).Tree(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Error("Tree on missing dir: want error, got nil")
	}
}

// TestWalkFilesStopsAtLimit proves the walk is bounded: without .gitignore
// nothing else keeps a dependency directory out of one RPC answer. walkLimit's
// own value is pinned as a literal — deriving it would make the test follow the
// constant instead of pinning it.
func TestWalkFilesStopsAtLimit(t *testing.T) {
	if walkLimit != 20000 {
		t.Errorf("walkLimit = %d, want 20000", walkLimit)
	}
	dir := t.TempDir()
	for i := range 12 {
		write(t, dir, fmt.Sprintf("f%02d.txt", i), "x")
	}
	files, err := walkFiles(dir, 10)
	if err != nil {
		t.Fatalf("walkFiles: %v", err)
	}
	if len(files) != 10 {
		t.Errorf("walkFiles = %d files, want 10", len(files))
	}
}

// TestReadFile proves a tracked text file's bytes come back verbatim.
func TestReadFile(t *testing.T) {
	repo, _ := initRepo(t)
	got, err := New(nil).ReadFile(repo, "a.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if got != "one\n" {
		t.Errorf("ReadFile = %q, want %q", got, "one\n")
	}
}

// TestReadFileRejectsEscape proves traversal and absolute paths never reach the
// filesystem, mirroring DiscardFile's guard.
func TestReadFileRejectsEscape(t *testing.T) {
	repo, _ := initRepo(t)
	for _, rel := range []string{"../outside.txt", "/etc/passwd", "a/../../b"} {
		if _, err := New(nil).ReadFile(repo, rel); err == nil {
			t.Errorf("ReadFile(%q): want error, got nil", rel)
		}
	}
}

// TestReadFileRejectsBinary proves a NUL-bearing file is refused rather than
// streamed into the text preview.
func TestReadFileRejectsBinary(t *testing.T) {
	repo, _ := initRepo(t)
	write(t, repo, "bin", "abc\x00def")
	if _, err := New(nil).ReadFile(repo, "bin"); err == nil {
		t.Error("ReadFile(binary): want error, got nil")
	}
}

// TestReadFileRejectsLarge proves a file above the size cap is refused.
func TestReadFileRejectsLarge(t *testing.T) {
	repo, _ := initRepo(t)
	write(t, repo, "big", strings.Repeat("x", maxReadFileSize+1))
	if _, err := New(nil).ReadFile(repo, "big"); err == nil {
		t.Error("ReadFile(oversize): want error, got nil")
	}
}

// TestIsBinaryWindow pins the edge of git's sniff window, the one thing the
// callers' own tests cannot see: they only ever feed a NUL near byte 0, so a
// wider or narrower window still passes them. The offsets are git's literal
// 8000 on purpose — deriving them from binarySniffBytes would make the test
// follow the constant instead of pinning it.
func TestIsBinaryWindow(t *testing.T) {
	nulAt := func(offset int) []byte {
		data := bytes.Repeat([]byte("x"), offset+1)
		data[offset] = 0
		return data
	}
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{"empty", nil, false},
		{"short text", []byte("package main\n"), false},
		{"short with NUL", []byte("ab\x00c"), true},
		{"last byte in window", nulAt(7999), true},
		{"first byte past window", nulAt(8000), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isBinary(tt.data); got != tt.want {
				t.Errorf("isBinary(%s) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

// TestReadFileMissing proves an absent (but path-valid) file is an error.
func TestReadFileMissing(t *testing.T) {
	repo, _ := initRepo(t)
	if _, err := New(nil).ReadFile(repo, "nope.txt"); err == nil {
		t.Error("ReadFile(missing): want error, got nil")
	}
}

func mkdir(t *testing.T, repo, rel string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(repo, rel), 0o755); err != nil {
		t.Fatal(err)
	}
}

func write(t *testing.T, repo, rel, content string) {
	t.Helper()
	full := filepath.Join(repo, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
