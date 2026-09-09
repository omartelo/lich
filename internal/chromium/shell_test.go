package chromium

import (
	"errors"
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
// exit, only inside the startup grace. A close (nil error) never falls back, a
// pinned window never does, and a crash after the grace is the window lifecycle
// ending. The grace is the literal half-minute the CHANGELOG promises, not the
// constant: a change to one must fail here.
func TestFallsBack(t *testing.T) {
	crashed := errors.New("signal: segmentation fault")
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
		{"pinned window crashes", stepPinned, crashed, time.Second, false},
	}
	for _, c := range cases {
		if got := fallsBack(c.step, c.err, c.elapsed); got != c.want {
			t.Errorf("%s: fallsBack = %v, want %v", c.name, got, c.want)
		}
	}
}

// TestRunFallback walks run with the launch faked, on both platforms' answers
// to a window dying at startup: macOS degrades to the tab (ErrNoShell reaches
// main.go), everywhere else the crash is the error the user sees.
func TestRunFallback(t *testing.T) {
	const shell = "/usr/local/lib/lich/shell/lich-shell"
	crashed := errors.New("exit status 1")
	cases := []struct {
		name        string
		machine     fakeEnv
		exit        error
		tabFallback bool
		wantErr     error
	}{
		{name: "bundled window dies, macOS opens a tab", machine: fakeEnv{shell: shell}, exit: crashed, tabFallback: true, wantErr: ErrNoShell},
		{name: "bundled window dies elsewhere", machine: fakeEnv{shell: shell}, exit: crashed, wantErr: crashed},
		{name: "bundled window closed by the user", machine: fakeEnv{shell: shell}, tabFallback: true},
		{name: "no window at all", machine: fakeEnv{}, tabFallback: true, wantErr: ErrNoShell},
		{
			name: "pinned window dies",
			machine: fakeEnv{
				installed: map[string]bool{"/opt/x/lich-shell": true},
				vars:      map[string]string{OverrideEnv: "/opt/x/lich-shell"},
				shell:     shell,
			},
			exit:        crashed,
			tabFallback: true,
			wantErr:     crashed,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var runs []string
			err := run(c.machine.env(), func(w Result) error {
				runs = append(runs, w.Path)
				return c.exit
			}, c.tabFallback)
			if !errors.Is(err, c.wantErr) {
				t.Fatalf("run = %v, want %v", err, c.wantErr)
			}
			if len(runs) > 1 {
				t.Fatalf("launched %v, want at most one window", runs)
			}
		})
	}
}

// The Homebrew cask's `lich` on PATH is a symlink into Lich.app, and
// os.Executable reports the link. Resolving it is what keeps a launch by name
// on lich's own window instead of dropping it to a system browser.
func TestFindShellResolvesASymlinkedBinary(t *testing.T) {
	// Resolved up front: macOS hands out temp dirs under /var, itself a link to
	// /private/var, so the paths this asserts on have to be the ones findShell
	// resolves to.
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	macOS := filepath.Join(root, "Lich.app", "Contents", "MacOS")
	if err := os.MkdirAll(macOS, 0o755); err != nil {
		t.Fatal(err)
	}
	window := filepath.Join(macOS, shellName)
	for _, f := range []string{filepath.Join(macOS, "lich"), window} {
		if err := os.WriteFile(f, nil, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(bin, "lich")
	if err := os.Symlink(filepath.Join(macOS, "lich"), link); err != nil {
		t.Skipf("symlinks unavailable: %v", err) // unprivileged Windows
	}

	if got := findShell(link, os.Stat); got != window {
		t.Fatalf("findShell = %q, want %q", got, window)
	}
}
