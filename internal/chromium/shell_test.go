package chromium

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// fakeInfo is the one thing findShell asks of a stat result.
type fakeInfo struct{ dir bool }

func (f fakeInfo) Name() string       { return "" }
func (f fakeInfo) Size() int64        { return 0 }
func (f fakeInfo) Mode() fs.FileMode  { return 0 }
func (f fakeInfo) ModTime() time.Time { return time.Time{} }
func (f fakeInfo) IsDir() bool        { return f.dir }
func (f fakeInfo) Sys() any           { return nil }

func statOf(files map[string]bool) func(string) (os.FileInfo, error) {
	return func(path string) (os.FileInfo, error) {
		dir, ok := files[path]
		if !ok {
			return nil, errors.New("not found")
		}
		return fakeInfo{dir: dir}, nil
	}
}

// TestShellPaths pins the two layouts the packages and a bare tarball produce;
// a change here moves nfpm.yaml and the AUR PKGBUILD with it.
func TestShellPaths(t *testing.T) {
	got := shellPaths(filepath.FromSlash("/usr/local/bin/lich"))
	want := []string{
		filepath.FromSlash("/usr/local/bin/shell/lich-shell"),
		filepath.FromSlash("/usr/local/lib/lich/shell/lich-shell"),
	}
	if !slices.Equal(got, want) {
		t.Fatalf("shellPaths = %v, want %v", got, want)
	}
}

func TestFindShellPrefersTheOneBesideTheBinary(t *testing.T) {
	beside := filepath.FromSlash("/opt/lich/shell/lich-shell")
	lib := filepath.FromSlash("/opt/lib/lich/shell/lich-shell")
	got := findShell(filepath.FromSlash("/opt/lich/lich"), statOf(map[string]bool{beside: false, lib: false}))
	if got != beside {
		t.Fatalf("findShell = %q, want %q", got, beside)
	}
}

func TestFindShellFallsBackToLib(t *testing.T) {
	lib := filepath.FromSlash("/usr/lib/lich/shell/lich-shell")
	got := findShell(filepath.FromSlash("/usr/bin/lich"), statOf(map[string]bool{lib: false}))
	if got != lib {
		t.Fatalf("findShell = %q, want %q", got, lib)
	}
}

func TestFindShellSkipsDirectoriesAndMissing(t *testing.T) {
	beside := filepath.FromSlash("/opt/lich/shell/lich-shell")
	if got := findShell(filepath.FromSlash("/opt/lich/lich"), statOf(map[string]bool{beside: true})); got != "" {
		t.Fatalf("findShell = %q, want none for a directory", got)
	}
	if got := findShell(filepath.FromSlash("/opt/lich/lich"), statOf(nil)); got != "" {
		t.Fatalf("findShell = %q, want none when nothing exists", got)
	}
}
