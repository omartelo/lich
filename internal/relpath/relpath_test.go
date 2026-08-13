package relpath

import (
	"os"
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
