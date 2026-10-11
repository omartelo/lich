package chromium

import (
	"errors"
	"slices"
	"testing"
)

// fakeEnv is a machine described by what is on it: which paths resolve, what
// the environment says, and whether a window sits beside the binary. Every
// rung of the ladder is reachable from here without a window built.
type fakeEnv struct {
	installed map[string]bool
	vars      map[string]string
	shell     string
}

func (f fakeEnv) env() Env {
	return Env{
		LookPath: func(name string) (string, error) {
			if f.installed[name] {
				return name, nil
			}
			return "", errors.New("not found")
		},
		Getenv: func(key string) string { return f.vars[key] },
		Shell:  func() string { return f.shell },
	}
}

func TestResolveBundledShell(t *testing.T) {
	machine := fakeEnv{shell: "/usr/local/lib/lich/shell/lich-shell"}
	got, err := Resolve(machine.env())
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Path != machine.shell || got.Step != stepShell {
		t.Fatalf("Resolve = %q (%s), want the bundled window", got.Path, got.Step)
	}
}

// TestResolvePinOutranksBundledShell: the pin is the user's word, and it wins
// over lich's own window — that is how `task dev` reaches the window it built.
func TestResolvePinOutranksBundledShell(t *testing.T) {
	machine := fakeEnv{
		installed: map[string]bool{"/home/u/lich/bin/shell/lich-shell": true},
		vars:      map[string]string{OverrideEnv: "/home/u/lich/bin/shell/lich-shell"},
		shell:     "/usr/local/lib/lich/shell/lich-shell",
	}
	got, err := Resolve(machine.env())
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Path != "/home/u/lich/bin/shell/lich-shell" || got.Step != stepPinned {
		t.Fatalf("Resolve = %q (%s), want the pinned window", got.Path, got.Step)
	}
}

// TestResolvePinnedMissingIsLoud proves a pinned window that is not there
// fails rather than falling through to the bundled one: the user named it, and
// silently opening a different one is what the override exists to rule out.
func TestResolvePinnedMissingIsLoud(t *testing.T) {
	machine := fakeEnv{
		vars:  map[string]string{OverrideEnv: "/opt/lich/lich-shell"},
		shell: "/usr/local/lib/lich/shell/lich-shell",
	}
	_, err := Resolve(machine.env())
	if err == nil {
		t.Fatal("want an error when the pinned window is missing")
	}
	if errors.Is(err, ErrNoShell) {
		t.Fatal("a pinned window that is missing must not report as ErrNoShell: that one degrades to a tab")
	}
}

func TestResolveNoShell(t *testing.T) {
	_, err := Resolve(fakeEnv{}.env())
	if !errors.Is(err, ErrNoShell) {
		t.Fatalf("Resolve error = %v, want ErrNoShell", err)
	}
}

func TestDescribe(t *testing.T) {
	got := Result{Path: "/usr/lib/lich/shell/lich-shell", Step: stepShell}.Describe()
	want := "/usr/lib/lich/shell/lich-shell (the bundled window)"
	if got != want {
		t.Fatalf("Describe = %q, want %q", got, want)
	}
}

func TestParseFlags(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		pinned   string
		noWindow bool
		extra    []string
	}{
		{name: "nothing"},
		{name: "separate value", args: []string{"--shell", "/opt/x/lich-shell"}, pinned: "/opt/x/lich-shell"},
		{name: "joined value", args: []string{"--shell=/opt/x/lich-shell"}, pinned: "/opt/x/lich-shell"},
		{
			name:   "both forms of argument",
			args:   []string{"--shell", "/opt/x/lich-shell", "--", "--ozone-platform=wayland"},
			pinned: "/opt/x/lich-shell",
			extra:  []string{"--ozone-platform=wayland"},
		},
		{
			name:  "passthrough only",
			args:  []string{"--", "--ozone-platform=wayland"},
			extra: []string{"--ozone-platform=wayland"},
		},
		// Everything after `--` belongs to the window, including a word that
		// spells lich's own flag.
		{
			name:  "flag after the separator is the window's",
			args:  []string{"--", "--shell=/opt/x/lich-shell"},
			extra: []string{"--shell=/opt/x/lich-shell"},
		},
		{name: "value missing", args: []string{"--shell"}},
		{name: "no window", args: []string{"--no-window"}, noWindow: true},
		{
			name:     "no window beside the others",
			args:     []string{"--no-window", "--shell=/opt/x/lich-shell", "--", "--ozone-platform=wayland"},
			pinned:   "/opt/x/lich-shell",
			noWindow: true,
			extra:    []string{"--ozone-platform=wayland"},
		},
		{name: "no window after the separator is the window's", args: []string{"--", "--no-window"}, extra: []string{"--no-window"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pinned, noWindow, extra := ParseFlags(tt.args)
			if pinned != tt.pinned {
				t.Fatalf("pinned = %q, want %q", pinned, tt.pinned)
			}
			if noWindow != tt.noWindow {
				t.Fatalf("noWindow = %v, want %v", noWindow, tt.noWindow)
			}
			if !slices.Equal(extra, tt.extra) {
				t.Fatalf("extra = %v, want %v", extra, tt.extra)
			}
		})
	}
}

// RelaunchArgs is ParseFlags' inverse for the window's own switches: a lich
// launched again with them opens its window with the same ones.
func TestRelaunchArgsRoundTripsThroughParseFlags(t *testing.T) {
	for _, extra := range [][]string{nil, {"--ozone-platform=x11"}, {"--ozone-platform=x11", "--remote-debugging-port=9334"}} {
		_, _, got := ParseFlags(RelaunchArgs(extra))
		if !slices.Equal(got, extra) {
			t.Errorf("ParseFlags(RelaunchArgs(%q)) = %q", extra, got)
		}
	}
	if args := RelaunchArgs(nil); args != nil {
		t.Errorf("RelaunchArgs(nil) = %q, want no arguments", args)
	}
}
