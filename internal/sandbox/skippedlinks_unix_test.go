//go:build !windows

package sandbox

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/omartelo/lich/internal/providers"
)

// link creates name under home pointing at a real file elsewhere, the shape a
// dotfile manager leaves behind.
func link(t *testing.T, home, name string) {
	t.Helper()
	target := filepath.Join(t.TempDir(), filepath.Base(name))
	if err := os.WriteFile(target, []byte("x"), 0o600); err != nil {
		t.Fatalf("write link target: %v", err)
	}
	path := filepath.Join(home, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatalf("symlink %s: %v", name, err)
	}
}

// A dotfile the profile lists and the home answers with a link: skipped from
// the mounts, and named — the whole point of the field.
func TestDescribeNamesSkippedLinks(t *testing.T) {
	clearHarnessEnv(t)
	home := t.TempDir()
	link(t, home, ".gitconfig")

	spec := Describe(providers.Claude, home, filepath.Join(home, "checkout"), "", nil, false)

	if slices.Contains(spec.Read, filepath.Join(home, ".gitconfig")) {
		t.Errorf("a symlinked dotfile was mounted: %v", spec.Read)
	}
	if !slices.Contains(spec.SkippedLinks, ".gitconfig") {
		t.Errorf("SkippedLinks = %v, want it to name .gitconfig", spec.SkippedLinks)
	}
}

// The grant's own file is the same trap: without known_hosts the ssh agent
// signs nothing, so a link there takes the whole grant down and has to say so.
func TestDescribeNamesSkippedKnownHosts(t *testing.T) {
	clearHarnessEnv(t)
	home := t.TempDir()
	link(t, home, filepath.Join(".ssh", "known_hosts"))

	spec := Describe(providers.Claude, home, filepath.Join(home, "checkout"), "", nil, true)

	want := filepath.Join(".ssh", "known_hosts")
	if !slices.Contains(spec.SkippedLinks, want) {
		t.Errorf("SkippedLinks = %v, want it to name %s", spec.SkippedLinks, want)
	}
}

// A real file is mounted, and a real file is not news: naming one would put a
// line on the card about a path the session can read.
func TestDescribeLeavesRealFilesUnnamed(t *testing.T) {
	clearHarnessEnv(t)
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, ".gitconfig"), []byte("[user]\n"), 0o600); err != nil {
		t.Fatalf("write .gitconfig: %v", err)
	}

	spec := Describe(providers.Claude, home, filepath.Join(home, "checkout"), "", nil, false)

	if !slices.Contains(spec.Read, filepath.Join(home, ".gitconfig")) {
		t.Errorf("a real dotfile was not mounted: %v", spec.Read)
	}
	if len(spec.SkippedLinks) != 0 {
		t.Errorf("SkippedLinks = %v, want none", spec.SkippedLinks)
	}
}

// The binaries are the exception the whole policy is written around: their
// symlink chains are walked and every hop's directory is mounted (BinaryDirs),
// so nothing the caller hands in as an extra read is a path the session is
// missing. Fed a link among them, the list still names none of it.
func TestDescribeLeavesBinaryDirsUnnamed(t *testing.T) {
	clearHarnessEnv(t)
	home := t.TempDir()
	link(t, home, ".tools")

	spec := Describe(providers.Claude, home, filepath.Join(home, "checkout"), "",
		[]string{filepath.Join(home, ".tools")}, false)

	if slices.Contains(spec.SkippedLinks, ".tools") {
		t.Errorf("a binary directory was named as skipped: %v", spec.SkippedLinks)
	}
}
