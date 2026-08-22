//go:build !windows

package sandbox

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// shim writes an executable at path, creating its directory.
func shim(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// The shape every agent installer uses: a name on PATH that is a symlink into a
// versioned store somewhere else. Both directories have to be mounted, and the
// symlink itself must never be — binding it is what fails the spawn.
func TestBinaryDirsFollowsAnInstallerSymlink(t *testing.T) {
	home := t.TempDir()
	store := filepath.Join(home, ".local", "share", "agent", "versions")
	bin := filepath.Join(home, ".local", "bin")
	shim(t, filepath.Join(store, "1.2.3"))
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Symlink(filepath.Join(store, "1.2.3"), filepath.Join(bin, "agent")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	t.Setenv("PATH", bin)

	want := []string{bin, store}
	if got := BinaryDirs("agent"); !slices.Equal(got, want) {
		t.Errorf("BinaryDirs = %v, want %v", got, want)
	}
	// Never the executable itself: bubblewrap resolves a mount destination
	// through symlinks and fails on a target that is not in the sandbox yet.
	for _, dir := range BinaryDirs("agent") {
		if filepath.Base(dir) == "agent" || filepath.Base(dir) == "1.2.3" {
			t.Errorf("an executable reached the mount list: %v", dir)
		}
	}
}

// A relative link target resolves against the directory holding the link, not
// against the process's working directory.
func TestBinaryDirsResolvesRelativeLinks(t *testing.T) {
	home := t.TempDir()
	shim(t, filepath.Join(home, "store", "tool-1.0"))
	bin := filepath.Join(home, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Symlink("../store/tool-1.0", filepath.Join(bin, "tool")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	t.Setenv("PATH", bin)

	want := []string{bin, filepath.Join(home, "store")}
	if got := BinaryDirs("tool"); !slices.Equal(got, want) {
		t.Errorf("BinaryDirs = %v, want %v", got, want)
	}
}

func TestBinaryDirsOfAPlainBinary(t *testing.T) {
	home := t.TempDir()
	bin := filepath.Join(home, "bin")
	shim(t, filepath.Join(bin, "plain"))
	t.Setenv("PATH", bin)

	if got := BinaryDirs("plain"); !slices.Equal(got, []string{bin}) {
		t.Errorf("BinaryDirs = %v, want %v", got, []string{bin})
	}
}

// Nothing on PATH answers to it: a spawn that would have failed outside the
// sandbox too, and not a reason to fail building one.
func TestBinaryDirsOfAMissingBinary(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if got := BinaryDirs("no-such-binary-anywhere"); got != nil {
		t.Errorf("BinaryDirs = %v, want nil", got)
	}
}

// A link that points at itself must not spin the spawn.
func TestBinaryDirsSurvivesASymlinkLoop(t *testing.T) {
	bin := t.TempDir()
	if err := os.Symlink(filepath.Join(bin, "b"), filepath.Join(bin, "a")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	if err := os.Symlink(filepath.Join(bin, "a"), filepath.Join(bin, "b")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	t.Setenv("PATH", bin)

	got := BinaryDirs(filepath.Join(bin, "a"))
	if len(got) > symlinkHops {
		t.Errorf("the walk did not stop: %d entries", len(got))
	}
	for _, dir := range got {
		if dir != bin {
			t.Errorf("BinaryDirs left the loop's directory: %v", got)
		}
	}
}

// A dotfile symlinked out of a dotfiles repository is refused, not followed: it
// is a mount destination bubblewrap resolves through (which fails the spawn when
// a parent directory is already mounted), and an instruction to reach outside
// the private home.
func TestDescribeRefusesSymlinkedDotfiles(t *testing.T) {
	home := t.TempDir()
	dotfiles := filepath.Join(home, "dotfiles")
	if err := os.MkdirAll(dotfiles, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dotfiles, "gitconfig"), []byte("[user]\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	link := filepath.Join(home, ".gitconfig")
	if err := os.Symlink(filepath.Join(dotfiles, "gitconfig"), link); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	// A real directory beside it, to prove the filter drops the link alone.
	if err := os.MkdirAll(filepath.Join(home, ".config"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	spec := Describe("claude", home, filepath.Join(home, "repo"), "", nil, false)
	if slices.Contains(spec.Read, link) {
		t.Errorf("a symlinked dotfile was mounted: %v", spec.Read)
	}
	if !slices.Contains(spec.Read, filepath.Join(home, ".config")) {
		t.Errorf("the filter dropped a real directory: %v", spec.Read)
	}
}
