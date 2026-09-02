package chromium

import (
	"errors"
	"path/filepath"
	"slices"
	"testing"
)

// fakeEnv is a machine described by what is on it: which names resolve, what
// the environment says, and what the two helpers print. Every rung of the
// ladder is reachable from here, which is the point — none of them needs a
// browser installed on the machine running the tests.
type fakeEnv struct {
	installed map[string]bool
	vars      map[string]string
	output    map[string]string
	names     []string
}

func (f fakeEnv) env() Env {
	return Env{
		LookPath: func(name string) (string, error) {
			if f.installed[name] {
				return "/usr/bin/" + name, nil
			}
			return "", errors.New("not found")
		},
		Getenv: func(key string) string { return f.vars[key] },
		Output: func(name string, args ...string) ([]byte, error) {
			out, ok := f.output[filepath.Base(name)]
			if !ok {
				return nil, errors.New("no such command")
			}
			return []byte(out), nil
		},
		Candidates: func() []string {
			if f.names != nil {
				return f.names
			}
			return chromiumNames
		},
	}
}

func TestResolvePinnedWins(t *testing.T) {
	machine := fakeEnv{
		installed: map[string]bool{"chromium": true, "vivaldi": true},
		vars:      map[string]string{OverrideEnv: "vivaldi"},
	}
	got, err := Resolve(machine.env())
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Path != "/usr/bin/vivaldi" {
		t.Fatalf("Resolve = %q, want the pinned browser even though chromium is installed", got.Path)
	}
	if got.Step != stepPinned {
		t.Fatalf("step = %q, want %q", got.Step, stepPinned)
	}
}

// TestResolvePinnedMissingIsLoud proves a pinned browser that is not there
// fails rather than falling through to the scan: the user named a browser, and
// silently opening a different one is what the override exists to rule out.
func TestResolvePinnedMissingIsLoud(t *testing.T) {
	machine := fakeEnv{
		installed: map[string]bool{"chromium": true},
		vars:      map[string]string{OverrideEnv: "/opt/vivaldi/vivaldi"},
	}
	_, err := Resolve(machine.env())
	if err == nil {
		t.Fatal("want an error when the pinned browser is missing")
	}
	if errors.Is(err, ErrNoBrowser) {
		t.Fatal("a pinned browser that is missing must not report as ErrNoBrowser: that one degrades to a tab")
	}
}

func TestResolvePrefersDesktopDefault(t *testing.T) {
	machine := fakeEnv{
		installed: map[string]bool{"chromium": true, "vivaldi-stable": true, "xdg-settings": true},
		output:    map[string]string{"xdg-settings": "vivaldi-stable.desktop\n"},
	}
	got, err := Resolve(machine.env())
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Path != "/usr/bin/vivaldi-stable" {
		t.Fatalf("Resolve = %q, want the desktop default over the scan order", got.Path)
	}
	if got.Step != stepDefault {
		t.Fatalf("step = %q, want %q", got.Step, stepDefault)
	}
}

// TestResolveSkipsNonChromiumDefault is the reason the known-name set exists:
// Firefox has no --app mode, so handing it the flag opens nothing at all.
func TestResolveSkipsNonChromiumDefault(t *testing.T) {
	machine := fakeEnv{
		installed: map[string]bool{"firefox": true, "chromium": true, "xdg-settings": true},
		output:    map[string]string{"xdg-settings": "firefox.desktop\n"},
	}
	got, err := Resolve(machine.env())
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Path != "/usr/bin/chromium" {
		t.Fatalf("Resolve = %q, want the scan to answer past a Firefox default", got.Path)
	}
}

// TestResolveReadsBrowserEnv proves the POSIX convention is honoured: a
// colon-separated list whose entries may carry a %s placeholder.
func TestResolveReadsBrowserEnv(t *testing.T) {
	machine := fakeEnv{
		installed: map[string]bool{"firefox": true, "brave-browser": true, "chromium": true},
		vars:      map[string]string{"BROWSER": "firefox %s:brave-browser %s"},
	}
	got, err := Resolve(machine.env())
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Path != "/usr/bin/brave-browser" {
		t.Fatalf("Resolve = %q, want the first chromium-family entry of $BROWSER", got.Path)
	}
}

func TestResolveScansInPreferenceOrder(t *testing.T) {
	machine := fakeEnv{installed: map[string]bool{"vivaldi": true, "brave": true}}
	got, err := Resolve(machine.env())
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Path != "/usr/bin/brave" {
		t.Fatalf("Resolve = %q, want the earlier candidate %q", got.Path, "/usr/bin/brave")
	}
	if got.Step != stepPath {
		t.Fatalf("step = %q, want %q", got.Step, stepPath)
	}
}

// TestResolveFlatpak covers the immutable-distribution machine: nothing
// Chromium-family on PATH, and the browser only installed as a Flatpak.
func TestResolveFlatpak(t *testing.T) {
	machine := fakeEnv{
		installed: map[string]bool{"flatpak": true, "firefox": true},
		vars:      map[string]string{"HOME": "/home/u"},
		output:    map[string]string{"flatpak": "org.gnome.Loupe\ncom.vivaldi.Vivaldi\n"},
	}
	got, err := Resolve(machine.env())
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Path != "/usr/bin/flatpak" || !slices.Equal(got.Prefix, []string{"run", "com.vivaldi.Vivaldi"}) {
		t.Fatalf("Resolve = %q %v, want a flatpak run of the installed browser", got.Path, got.Prefix)
	}
	// The profile has to land inside the sandbox's own directory, and keep the
	// name it had so the dev shell still gets a profile of its own.
	want := "/home/u/.var/app/com.vivaldi.Vivaldi/config/chromium-profile-dev"
	if dir := got.ProfileDir("/home/u/.config/lich/chromium-profile-dev"); dir != want {
		t.Fatalf("ProfileDir = %q, want %q", dir, want)
	}
}

// TestResolveFlatpakNeedsHome proves the rung is skipped rather than launched
// at a profile directory it cannot name — which would be a window that never
// opens, the exact failure this file answers.
func TestResolveFlatpakNeedsHome(t *testing.T) {
	machine := fakeEnv{
		installed: map[string]bool{"flatpak": true},
		output:    map[string]string{"flatpak": "com.vivaldi.Vivaldi\n"},
	}
	if _, err := Resolve(machine.env()); !errors.Is(err, ErrNoBrowser) {
		t.Fatalf("Resolve error = %v, want ErrNoBrowser", err)
	}
}

func TestResolveNoBrowser(t *testing.T) {
	machine := fakeEnv{installed: map[string]bool{"firefox": true}}
	_, err := Resolve(machine.env())
	if !errors.Is(err, ErrNoBrowser) {
		t.Fatalf("Resolve error = %v, want ErrNoBrowser — the caller degrades to a tab on that one", err)
	}
}

// TestProfileDirUnchanged proves the relocation is Flatpak's alone: every other
// browser reads the directory lich chose.
func TestProfileDirUnchanged(t *testing.T) {
	dir := "/home/u/.config/lich/chromium-profile"
	if got := (Result{Path: "/usr/bin/chromium"}).ProfileDir(dir); got != dir {
		t.Fatalf("ProfileDir = %q, want %q", got, dir)
	}
}

func TestDescribe(t *testing.T) {
	got := Result{Path: "/usr/bin/flatpak", Prefix: []string{"run", "com.brave.Browser"}, Step: stepFlatpak}.Describe()
	want := "/usr/bin/flatpak run com.brave.Browser (flatpak)"
	if got != want {
		t.Fatalf("Describe = %q, want %q", got, want)
	}
}

func TestParseFlags(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		pinned string
		extra  []string
	}{
		{name: "nothing"},
		{name: "separate value", args: []string{"--browser", "vivaldi"}, pinned: "vivaldi"},
		{name: "joined value", args: []string{"--browser=/opt/x/x"}, pinned: "/opt/x/x"},
		{
			name:   "both forms of argument",
			args:   []string{"--browser", "brave", "--", "--ozone-platform=wayland"},
			pinned: "brave",
			extra:  []string{"--ozone-platform=wayland"},
		},
		{
			name:  "passthrough only",
			args:  []string{"--", "--ozone-platform=wayland"},
			extra: []string{"--ozone-platform=wayland"},
		},
		// Everything after `--` belongs to the browser, including a word that
		// spells one of lich's own flags.
		{
			name:  "flag after the separator is the browser's",
			args:  []string{"--", "--browser=chromium"},
			extra: []string{"--browser=chromium"},
		},
		{name: "value missing", args: []string{"--browser"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pinned, extra := ParseFlags(tt.args)
			if pinned != tt.pinned {
				t.Fatalf("pinned = %q, want %q", pinned, tt.pinned)
			}
			if !slices.Equal(extra, tt.extra) {
				t.Fatalf("extra = %v, want %v", extra, tt.extra)
			}
		})
	}
}

// TestResolveDesktopAlias covers the .desktop id that is not its executable's
// name: helium.desktop launches helium-browser. Without the alias the default
// misses and lich opens some other installed browser — the user's own choice
// ignored, with nothing saying why.
func TestResolveDesktopAlias(t *testing.T) {
	machine := fakeEnv{
		installed: map[string]bool{"helium-browser": true, "chromium": true, "xdg-settings": true},
		output:    map[string]string{"xdg-settings": "helium.desktop\n"},
	}
	got, err := Resolve(machine.env())
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Path != "/usr/bin/helium-browser" {
		t.Fatalf("Resolve = %q, want the aliased default browser", got.Path)
	}
}
