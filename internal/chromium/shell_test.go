package chromium

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

// statOf is os.Stat over a map of paths, true for a directory; a slash-rooted
// path is what shellPaths builds on every OS, fstest wants it unrooted.
func statOf(files map[string]bool) func(string) (os.FileInfo, error) {
	fsys := fstest.MapFS{}
	for path, dir := range files {
		var mode fs.FileMode
		if dir {
			mode = fs.ModeDir
		}
		fsys[strings.TrimPrefix(filepath.ToSlash(path), "/")] = &fstest.MapFile{Mode: mode}
	}
	return func(path string) (os.FileInfo, error) {
		return fs.Stat(fsys, strings.TrimPrefix(filepath.ToSlash(path), "/"))
	}
}

// TestShellPaths pins the three layouts the packages, a bare tarball and the
// macOS bundle produce; a change here moves nfpm.yaml, the AUR PKGBUILD,
// lich.iss, build/darwin/bundle.sh and appupdate.windowed with it. The
// binary's name is the one thing taken from the constant: its suffix is the
// OS's, not the layout's.
func TestShellPaths(t *testing.T) {
	got := shellPaths(filepath.FromSlash("/usr/local/bin/lich"))
	want := []string{
		filepath.FromSlash("/usr/local/bin/shell/" + shellName),
		filepath.FromSlash("/usr/local/bin/" + shellName),
		filepath.FromSlash("/usr/local/lib/lich/shell/" + shellName),
	}
	if !slices.Equal(got, want) {
		t.Fatalf("shellPaths = %v, want %v", got, want)
	}
}

func TestFindShellPrefersTheOneBesideTheBinary(t *testing.T) {
	beside := filepath.FromSlash("/opt/lich/shell/" + shellName)
	lib := filepath.FromSlash("/opt/lib/lich/shell/" + shellName)
	got := findShell(filepath.FromSlash("/opt/lich/lich"), statOf(map[string]bool{beside: false, lib: false}))
	if got != beside {
		t.Fatalf("findShell = %q, want %q", got, beside)
	}
}

func TestFindShellInTheAppBundle(t *testing.T) {
	beside := filepath.FromSlash("/Applications/Lich.app/Contents/MacOS/" + shellName)
	got := findShell(filepath.FromSlash("/Applications/Lich.app/Contents/MacOS/lich"), statOf(map[string]bool{beside: false}))
	if got != beside {
		t.Fatalf("findShell = %q, want %q", got, beside)
	}
}

func TestFindShellFallsBackToLib(t *testing.T) {
	lib := filepath.FromSlash("/usr/lib/lich/shell/" + shellName)
	got := findShell(filepath.FromSlash("/usr/bin/lich"), statOf(map[string]bool{lib: false}))
	if got != lib {
		t.Fatalf("findShell = %q, want %q", got, lib)
	}
}

func TestFindShellSkipsDirectoriesAndMissing(t *testing.T) {
	beside := filepath.FromSlash("/opt/lich/shell/" + shellName)
	if got := findShell(filepath.FromSlash("/opt/lich/lich"), statOf(map[string]bool{beside: true})); got != "" {
		t.Fatalf("findShell = %q, want none for a directory", got)
	}
	if got := findShell(filepath.FromSlash("/opt/lich/lich"), statOf(nil)); got != "" {
		t.Fatalf("findShell = %q, want none when nothing exists", got)
	}
}

// TestFallsBack pins the conditions: only the bundled window, only an error
// exit, only inside the startup grace, and only once the window ran. A close
// (nil error) never falls back, a pinned browser never does, a crash after the
// grace is the window lifecycle ending, and a profile directory that could not
// be made is the next rung's failure too. The grace is the literal half-minute
// the CHANGELOG promises, not the constant: a change to one must fail here.
func TestFallsBack(t *testing.T) {
	crashed := errors.New("signal: segmentation fault")
	noDir := fmt.Errorf("%w: %w", errProfileDir, fs.ErrPermission)
	cases := []struct {
		name    string
		step    string
		err     error
		elapsed time.Duration
		want    bool
	}{
		{"shell crashes at startup", stepShell, crashed, 2 * time.Second, true},
		{"shell crashes at startup, core dump written", stepShell, crashed, 19 * time.Second, true},
		{"shell crashes just inside the grace", stepShell, crashed, 30*time.Second - time.Millisecond, true},
		{"shell closed by the user", stepShell, nil, 2 * time.Second, false},
		{"shell crashes after the grace", stepShell, crashed, 30 * time.Second, false},
		{"shell profile dir cannot be made", stepShell, noDir, time.Second, false},
		{"pinned browser crashes", stepPinned, crashed, time.Second, false},
		{"system browser crashes", stepDefault, crashed, time.Second, false},
	}
	for _, c := range cases {
		if got := fallsBack(c.step, c.err, c.elapsed); got != c.want {
			t.Errorf("%s: fallsBack = %v, want %v", c.name, got, c.want)
		}
	}
}

// TestRunLadder walks Run's ladder with the launch faked: which browsers get
// started, in what order, what comes back, and whether the desktop is told.
func TestRunLadder(t *testing.T) {
	const shell = "/usr/local/lib/lich/shell/lich-shell"
	crashed := errors.New("exit status 1")
	noDir := fmt.Errorf("%w: %w", errProfileDir, fs.ErrPermission)
	withChromium := map[string]bool{"chromium": true}
	cases := []struct {
		name     string
		machine  fakeEnv
		exits    map[string]error
		wantErr  error
		wantRuns []string
		wantNote bool
	}{
		{
			name:     "bundled window dies, the machine's browser answers",
			machine:  fakeEnv{installed: withChromium, shell: shell},
			exits:    map[string]error{shell: crashed},
			wantRuns: []string{shell, "/usr/bin/chromium"},
			wantNote: true,
		},
		{
			name:     "bundled window dies, no browser at all",
			machine:  fakeEnv{shell: shell},
			exits:    map[string]error{shell: crashed},
			wantErr:  ErrNoBrowser,
			wantRuns: []string{shell},
		},
		{
			name:     "bundled window closed by the user",
			machine:  fakeEnv{installed: withChromium, shell: shell},
			wantRuns: []string{shell},
		},
		{
			name: "pinned browser dies",
			machine: fakeEnv{
				installed: map[string]bool{"vivaldi": true},
				vars:      map[string]string{OverrideEnv: "vivaldi"},
				shell:     shell,
			},
			exits:    map[string]error{"/usr/bin/vivaldi": crashed},
			wantErr:  crashed,
			wantRuns: []string{"/usr/bin/vivaldi"},
		},
		{
			name:     "profile directory cannot be made",
			machine:  fakeEnv{installed: withChromium, shell: shell},
			exits:    map[string]error{shell: noDir},
			wantErr:  errProfileDir,
			wantRuns: []string{shell},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var runs []string
			var note string
			err := run(c.machine.env(), func(b Result) error {
				runs = append(runs, b.Path)
				return c.exits[b.Path]
			}, func(text string) { note = text })
			if !errors.Is(err, c.wantErr) {
				t.Fatalf("run = %v, want %v", err, c.wantErr)
			}
			if !slices.Equal(runs, c.wantRuns) {
				t.Fatalf("launched %v, want %v", runs, c.wantRuns)
			}
			if (note != "") != c.wantNote {
				t.Fatalf("notification %q, want one: %v", note, c.wantNote)
			}
			if c.wantNote && !strings.Contains(note, "chromium") {
				t.Fatalf("notification %q does not name the browser", note)
			}
		})
	}
}
