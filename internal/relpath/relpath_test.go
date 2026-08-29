package relpath

import (
	"os"
	"path/filepath"
	"testing"
)

// TestValidateRejectsRootedPaths pins the guard itself. The callers' escape
// tests pass on Windows for the wrong reason — the joined path simply misses on
// disk — so the rule is asserted here directly: a path starting at a root is
// never a work-tree path, whether or not it carries a volume name.
// "/etc/passwd" is the case Windows used to wave through: filepath.IsAbs wants
// a volume name there, so a leading separator alone read as relative.
func TestValidateRejectsRootedPaths(t *testing.T) {
	rooted := []string{"/etc/passwd", "/"}
	if os.IsPathSeparator('\\') {
		rooted = append(rooted, `\Windows\System32\config`, `C:\Windows`)
	}
	for _, rel := range rooted {
		if err := Validate(rel); err == nil {
			t.Errorf("Validate(%q): want error, got nil", rel)
		}
	}
}

// TestValidateRejectsTraversal proves a path cannot climb out of the root it
// will be joined onto, including the forms Clean leaves behind.
func TestValidateRejectsTraversal(t *testing.T) {
	for _, rel := range []string{"..", "../etc/passwd", "a/../../b", "./.."} {
		if err := Validate(rel); err == nil {
			t.Errorf("Validate(%q): want error, got nil", rel)
		}
	}
}

// TestValidateRejectsTheRoot proves the work-tree root itself is not a path to
// a file. Every one of these cleans to "." — the empty string included — and a
// caller that takes it for one file operates on the whole repository.
func TestValidateRejectsTheRoot(t *testing.T) {
	for _, rel := range []string{".", "", "./", "a/.."} {
		if err := Validate(rel); err == nil {
			t.Errorf("Validate(%q): want error, got nil", rel)
		}
	}
}

// TestValidateAcceptsWorkTreePaths proves the ordinary shapes still pass —
// including a traversal that stays inside the root.
func TestValidateAcceptsWorkTreePaths(t *testing.T) {
	for _, rel := range []string{"a.txt", "internal/rpc/rpc.go", "src/main.go", "a/b/../c.txt"} {
		if err := Validate(rel); err != nil {
			t.Errorf("Validate(%q): want nil, got %v", rel, err)
		}
	}
}

// linkTree lays out a checkout holding one real file, one link to it, and one
// link pointing out of the tree, and returns the checkout and the outside file.
// It skips where the OS refuses to make a symlink at all: an unprivileged
// Windows runner without developer mode, where os.Symlink is the thing that
// fails rather than the rule under test.
func linkTree(t *testing.T) (root, outside string) {
	t.Helper()
	base := t.TempDir()
	root = filepath.Join(base, "checkout")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatalf("mkdir checkout: %v", err)
	}
	outside = filepath.Join(base, "secret.txt")
	for path, content := range map[string]string{
		outside:                         "not the repository's\n",
		filepath.Join(root, "real.txt"): "the repository's\n",
	} {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	if err := os.Symlink("real.txt", filepath.Join(root, "inside.txt")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape.txt")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	return root, outside
}

// TestResolveRefusesALinkOutOfTheTree is the rule Validate cannot state: the
// path is lexically a work-tree path and the escape is on disk. git records
// such an entry as its target text, so a reader that followed it would print a
// file the diff beside it never mentions.
func TestResolveRefusesALinkOutOfTheTree(t *testing.T) {
	root, _ := linkTree(t)
	if err := Validate("escape.txt"); err != nil {
		t.Fatalf("Validate(%q): the lexical rule is meant to pass here, got %v", "escape.txt", err)
	}
	if got, err := Resolve(root, "escape.txt"); err == nil {
		t.Errorf("Resolve(escape.txt) = %q, want an error", got)
	}
}

// TestResolveFollowsALinkInsideTheTree proves the refusal is about leaving, not
// about links: a checkout that ships one (this repository's own AGENTS.md is a
// link to CLAUDE.md) still reads.
func TestResolveFollowsALinkInsideTheTree(t *testing.T) {
	root, _ := linkTree(t)
	got, err := Resolve(root, "inside.txt")
	if err != nil {
		t.Fatalf("Resolve(inside.txt): %v", err)
	}
	if want := filepath.Join(root, "real.txt"); got != want {
		t.Errorf("Resolve(inside.txt) = %q, want %q", got, want)
	}
}

// TestResolveAcceptsARootReachedThroughALink is the case that makes resolving
// BOTH sides load-bearing: macOS hands out /var, which is /private/var, so a
// resolved target measured against an unresolved root reads every file in the
// checkout as an escape.
func TestResolveAcceptsARootReachedThroughALink(t *testing.T) {
	root, _ := linkTree(t)
	linked := filepath.Join(t.TempDir(), "via-link")
	if err := os.Symlink(root, linked); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := Resolve(linked, "real.txt"); err != nil {
		t.Errorf("Resolve through a linked root: %v", err)
	}
}

// TestResolveKeepsTheLexicalRule proves Resolve did not trade the cheap check
// away: a traversal is refused before the filesystem is touched at all.
func TestResolveKeepsTheLexicalRule(t *testing.T) {
	root, _ := linkTree(t)
	for _, rel := range []string{"..", "../secret.txt", ".", ""} {
		if _, err := Resolve(root, rel); err == nil {
			t.Errorf("Resolve(%q): want error, got nil", rel)
		}
	}
}
