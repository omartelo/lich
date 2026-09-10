package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestIdentifyMatchesThePicker proves a path typed at a command line lands on
// the same identity the dialog would have produced for that directory: the id is
// what a project's sessions and its worktree directory hang off, so a second one
// for the same directory is a second project on top of the first.
func TestIdentifyMatchesThePicker(t *testing.T) {
	dir := t.TempDir()

	p, err := Identify(dir)
	if err != nil {
		t.Fatalf("Identify: %v", err)
	}
	if want := newProject(dir); *p != *want {
		t.Errorf("Identify(%q) = %+v, want %+v", dir, p, want)
	}
}

// TestIdentifyNormalizes proves the spellings of one directory that a person or
// an agent actually types all reach that one project. The id is a hash of the
// path string, so anything left un-normalized here opens a duplicate.
func TestIdentifyNormalizes(t *testing.T) {
	dir := t.TempDir()
	inner := filepath.Join(dir, "repo")
	if err := os.Mkdir(inner, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	home := t.TempDir()
	// os.UserHomeDir reads HOME on Unix and USERPROFILE on Windows.
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	want := newProject(inner).ID
	for _, spelling := range []string{
		inner,
		inner + string(filepath.Separator),
		filepath.Join(dir, "repo", "..", "repo"),
		" " + inner + " ",
	} {
		got, err := Identify(spelling)
		if err != nil {
			t.Fatalf("Identify(%q): %v", spelling, err)
		}
		if got.ID != want {
			t.Errorf("Identify(%q).ID = %q, want %q", spelling, got.ID, want)
		}
	}

	// A tilde reaches lich literally through an MCP call: there is no shell at
	// that end to expand it.
	got, err := Identify("~")
	if err != nil {
		t.Fatalf("Identify(~): %v", err)
	}
	if got.Path != filepath.Clean(home) {
		t.Errorf("Identify(~).Path = %q, want %q", got.Path, home)
	}
}

// TestIdentifyRefuses proves the boundary answers rather than opening the wrong
// project. A relative path is the one that matters: it resolves against the lich
// window's own directory, not the caller's, so accepting it would silently open
// somewhere else.
func TestIdentifyRefuses(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "README.md")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	for _, tc := range []struct{ name, path, want string }{
		{"relative", filepath.Join(".", "repo"), "relative path"},
		{"empty", "   ", "no path given"},
		{"missing", filepath.Join(dir, "nope"), "no directory at"},
		{"file", file, "is a file"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, err := Identify(tc.path)
			if err == nil {
				t.Fatalf("Identify(%q) = %+v, want an error", tc.path, p)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Identify(%q) said %q, want it to name %q", tc.path, err, tc.want)
			}
		})
	}
}
