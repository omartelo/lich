package project

import (
	"bytes"
	"fmt"
	"maps"
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

	tree, err := New(nil).Tree(repo)
	if err != nil {
		t.Fatalf("Tree: %v", err)
	}
	got := strings.Join(tree.Files, ",")
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
	tree, err := New(nil).Tree(repo)
	if err != nil {
		t.Fatalf("Tree: %v", err)
	}
	if slices.Contains(tree.Files, "a.txt") {
		t.Errorf("Tree = %v, want a.txt dropped", tree.Files)
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
	tree, err := New(nil).Tree(dir)
	if err != nil {
		t.Fatalf("Tree: %v", err)
	}
	if got, want := strings.Join(tree.Files, ","), "b.txt,src/main.go"; got != want {
		t.Errorf("Tree = %q, want %q", got, want)
	}
	if tree.Cut {
		t.Error("Tree.Cut = true on a walk that reached the end, want false")
	}
	if got := strings.Join(tree.Hidden, ","); got != ".git" {
		t.Errorf("Tree.Hidden = %q, want %q", got, ".git")
	}
}

// TestTreeReportsBrokenRepo proves the gate only routes a path away from git
// when git itself disowns it: a repository whose index is corrupt still passes
// rev-parse, so its ls-files failure is the answer. Walking it instead would
// draw a plausible tree over a repository nobody can list.
func TestTreeReportsBrokenRepo(t *testing.T) {
	repo, _ := initRepo(t)
	write(t, repo, ".git/index", "not an index")
	tree, err := New(nil).Tree(repo)
	if err == nil {
		t.Fatalf("Tree on a corrupt index = %v, want an error", tree.Files)
	}
}

// TestTreeMissingDir proves an absent directory is still an error, so the panel
// says so instead of drawing an empty tree over a folder that is gone.
func TestTreeMissingDir(t *testing.T) {
	if _, err := New(nil).Tree(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Error("Tree on missing dir: want error, got nil")
	}
}

// TestWalkFilesStopsAtLimit proves the walk is bounded and says so: walkIgnore
// is a name list, not a .gitignore, so the cap is still what stands between a
// huge folder and one RPC answer, and Cut is what lets the panel admit the
// listing is partial. walkLimit's own value is pinned as a literal (deriving
// it would make the test follow the constant instead of pinning it).
func TestWalkFilesStopsAtLimit(t *testing.T) {
	if walkLimit != 20000 {
		t.Errorf("walkLimit = %d, want 20000", walkLimit)
	}
	dir := t.TempDir()
	for i := range 12 {
		write(t, dir, fmt.Sprintf("f%02d.txt", i), "x")
	}
	tree, err := walkFiles(dir, 10)
	if err != nil {
		t.Fatalf("walkFiles: %v", err)
	}
	if len(tree.Files) != 10 {
		t.Errorf("walkFiles = %d files, want 10", len(tree.Files))
	}
	if !tree.Cut {
		t.Error("walkFiles.Cut = false on a listing that stopped at the cap, want true")
	}
}

// TestWalkIgnoresDependencyDirs proves the plain-folder walk leaves the
// machine-written trees out at any depth, and only those: a file whose *own*
// name matches an ignored directory is a file, and a directory whose name only
// contains one is not the one being skipped.
func TestWalkIgnoresDependencyDirs(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "main.go", "package main\n")
	write(t, dir, "node_modules/left-pad/index.js", "x")
	write(t, dir, "src/node_modules/dep/index.js", "x")
	write(t, dir, "src/app/dist/bundle.js", "x")
	write(t, dir, "api/__pycache__/mod.pyc", "x")
	write(t, dir, "web/.venv/lib/site.py", "x")
	write(t, dir, "go/vendor/dep/dep.go", "x")
	write(t, dir, "rs/target/debug/bin", "x")
	write(t, dir, "cc/build/out.o", "x")
	write(t, dir, "py/venv/bin/python", "x")
	write(t, dir, "app/.cache/blob", "x")
	write(t, dir, "dist.txt", "not a directory")
	write(t, dir, "build-tools/run.sh", "not build/")

	tree, err := walkFiles(dir, walkLimit)
	if err != nil {
		t.Fatalf("walkFiles: %v", err)
	}
	want := "build-tools/run.sh,dist.txt,main.go"
	if got := strings.Join(tree.Files, ","); got != want {
		t.Errorf("walkFiles = %q, want %q", got, want)
	}
	// Every ignored name this folder actually had, once each: node_modules sat
	// at two depths, and .git was not here at all.
	wantHidden := ".cache,.venv,__pycache__,build,dist,node_modules,target,vendor,venv"
	if got := strings.Join(tree.Hidden, ","); got != wantHidden {
		t.Errorf("walkFiles.Hidden = %q, want %q", got, wantHidden)
	}
}

// TestWalkHiddenNamesOnlyWhatIsThere proves Hidden is the folder's own answer
// and not the ignore set recited back: it is what the panel prints, so a name
// in it that is not on disk sends the reader looking for a directory that was
// never there.
func TestWalkHiddenNamesOnlyWhatIsThere(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "main.go", "package main\n")
	write(t, dir, "dist/out.js", "x")

	tree, err := walkFiles(dir, walkLimit)
	if err != nil {
		t.Fatalf("walkFiles: %v", err)
	}
	if got := strings.Join(tree.Hidden, ","); got != "dist" {
		t.Errorf("walkFiles.Hidden = %q, want %q", got, "dist")
	}

	clean := t.TempDir()
	write(t, clean, "main.go", "package main\n")
	bare, err := walkFiles(clean, walkLimit)
	if err != nil {
		t.Fatalf("walkFiles: %v", err)
	}
	if len(bare.Hidden) != 0 {
		t.Errorf("walkFiles.Hidden = %v on a folder with nothing to skip, want empty", bare.Hidden)
	}
}

// TestWalkIgnoreNames pins the set itself. The walk is the only filter a plain
// folder gets, so a name silently dropped from here is a dependency tree back
// in the panel, and a name silently added is a directory of somebody's source
// gone from it.
func TestWalkIgnoreNames(t *testing.T) {
	want := []string{
		".cache", ".git", ".venv", "__pycache__", "build",
		"dist", "node_modules", "target", "vendor", "venv",
	}
	got := slices.Sorted(maps.Keys(walkIgnore))
	if !slices.Equal(got, want) {
		t.Errorf("walkIgnore = %v, want %v", got, want)
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

// TestReadFileRejectsALinkOutOfTheCheckout pins ReadFile on relpath.Resolve
// rather than the lexical Validate. Every other escape test here is refused
// before the filesystem is reached; this one is a legal work-tree path whose
// escape exists only on disk, so it is the one that fails if this caller is
// ever moved back to the cheaper guard. The link is the shape a repository can
// genuinely ship — git records it as its target text, and a preview that
// followed it would print a file the diff beside it never mentions.
func TestReadFileRejectsALinkOutOfTheCheckout(t *testing.T) {
	repo, _ := initRepo(t)
	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("not the repository's\n"), 0o600); err != nil {
		t.Fatalf("write outside: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(repo, "escape.txt")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if got, err := New(nil).ReadFile(repo, "escape.txt"); err == nil {
		t.Errorf("ReadFile(escape.txt) = %q, want an error", got)
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
	} else if got := err.Error(); strings.Contains(got, repo) || !strings.HasPrefix(got, "stat nope.txt:") {
		t.Errorf("ReadFile(missing) error = %q, want only the relative path", got)
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
