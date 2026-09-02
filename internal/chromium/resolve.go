package chromium

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ErrNoBrowser is the machine having no Chromium-family browser at all. It is a
// sentinel because the caller answers it differently from every other
// resolution failure: without one lich still runs, in a plain tab of whatever
// browser is installed (main.go's openWithoutWindow), while a browser the user
// *named* and lich could not find is an error the user has to see.
var ErrNoBrowser = errors.New(
	"no chromium-family browser found — install chromium, chrome, brave, vivaldi or edge, " +
		"or point LICH_BROWSER at one")

// OverrideEnv pins the browser to launch, by PATH name or by path. The
// --browser flag is carried in it rather than passed down as an argument, so
// the restart successor inherits the choice and `lich doctor` reports the
// browser a launch here would actually open.
const OverrideEnv = "LICH_BROWSER"

// The rung of the ladder that answered, as the diagnostics name it.
const (
	stepPinned  = "pinned by LICH_BROWSER"
	stepDefault = "the desktop's default browser"
	stepPath    = "installed"
	stepFlatpak = "flatpak"
)

// probeTimeout bounds the helpers resolution shells out to. They run on the
// path to the window: one that hangs would hang the launch, and a launch that
// opens nothing is the failure this whole file exists to answer.
const probeTimeout = 3 * time.Second

// Env is everything resolution reads from the machine. Every probe is a field
// so the whole ladder is table-testable on one machine with no browser
// installed — the exec at the end is the only part that is not.
type Env struct {
	LookPath   func(name string) (string, error)
	Getenv     func(key string) string
	Output     func(name string, args ...string) ([]byte, error)
	Candidates func() []string
}

// RealEnv reaches the machine lich is running on.
func RealEnv() Env {
	return Env{
		LookPath: exec.LookPath,
		Getenv:   os.Getenv,
		Output: func(name string, args ...string) ([]byte, error) {
			ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
			defer cancel()
			cmd := exec.CommandContext(ctx, name, args...)
			// The context kills the process; without this the read of its
			// output pipe outlives the kill, and the timeout bounds nothing.
			cmd.WaitDelay = time.Second
			return cmd.Output()
		},
		Candidates: browserCandidates,
	}
}

// Result is a resolved launch: which executable, what has to precede lich's own
// arguments, and where the profile may live.
type Result struct {
	// Path is the executable to run.
	Path string
	// Prefix goes before lich's arguments — `run <app-id>` for Flatpak, empty
	// for a browser installed on the machine itself.
	Prefix []string
	// Step names the rung that answered, for `lich doctor` and `lich rage`.
	Step string
	// ProfileRoot relocates the Chromium profile directory, and is set only
	// where the browser cannot see the one lich keeps under its config dir: a
	// Flatpak sandbox reliably writes under its own ~/.var/app/<id> and little
	// else. Empty means the caller's directory stands.
	ProfileRoot string
}

// Describe is the resolution as a diagnostic prints it: the command, and the
// rung that produced it.
func (r Result) Describe() string {
	command := strings.Join(append([]string{r.Path}, r.Prefix...), " ")
	return fmt.Sprintf("%s (%s)", command, r.Step)
}

// ProfileDir is where this browser's profile goes, given the directory lich
// would use. Only a sandboxed browser moves it, and it keeps the directory's
// name so the dev shell still gets a profile of its own.
func (r Result) ProfileDir(dir string) string {
	if r.ProfileRoot == "" {
		return dir
	}
	return filepath.Join(r.ProfileRoot, filepath.Base(dir))
}

// chromiumNames are the Chromium-family executables lich knows, in preference
// order. It is the Linux/BSD candidate list (candidates_unix.go) and the set
// that decides whether a browser lich did *not* pick — the desktop's default,
// $BROWSER — can be driven in --app mode at all.
var chromiumNames = []string{
	"chromium",
	"chromium-browser",
	"google-chrome",
	"google-chrome-stable",
	"google-chrome-beta",
	"helium-browser",
	"brave",
	"brave-browser",
	"vivaldi",
	"vivaldi-stable",
	"microsoft-edge",
	"microsoft-edge-stable",
	"thorium-browser",
	"ungoogled-chromium",
	"chrome",
}

// knownChromium is chromiumNames as a set, plus the spellings the other two
// platforms install under. Membership is what stops lich handing --app= to a
// Firefox the user set as their default: Firefox has no app mode, so the flag
// is ignored and the launch opens nothing.
var knownChromium = func() map[string]bool {
	set := map[string]bool{"msedge": true, "brave-browser-stable": true}
	for _, name := range chromiumNames {
		set[name] = true
	}
	return set
}()

// desktopExec is the .desktop ids whose name is not the executable's. Only the
// ones that differ belong here: the freedesktop id and the binary agree for
// every other browser lich knows (chromium.desktop, vivaldi-stable.desktop,
// microsoft-edge.desktop). The general answer is parsing the entry's Exec= line
// out of XDG_DATA_DIRS, which is a lot of code and a lot of quoting rules for a
// map that has one row.
var desktopExec = map[string]string{"helium": "helium-browser"}

// flatpakIDs are the Chromium-family Flatpak applications, in the same
// preference order as the PATH scan.
var flatpakIDs = []string{
	"org.chromium.Chromium",
	"com.google.Chrome",
	"com.brave.Browser",
	"com.vivaldi.Vivaldi",
	"com.microsoft.Edge",
}

// Resolve finds the browser to be the window, climbing the ladder in order: the
// browser the user pinned, the desktop's default when it is one lich can drive,
// this platform's candidates, then Flatpak. It returns ErrNoBrowser when the
// machine has none, which is a fallback and not a failure — see that variable.
func Resolve(env Env) (Result, error) {
	if pinned := strings.TrimSpace(env.Getenv(OverrideEnv)); pinned != "" {
		path, err := env.LookPath(pinned)
		if err != nil {
			// Loud, and never a fall-through to the scan below: the user named
			// this browser, and quietly opening a different one is the whole
			// class of bug the override exists to rule out.
			return Result{}, fmt.Errorf("%s=%s: %w", OverrideEnv, pinned, err)
		}
		return Result{Path: path, Step: stepPinned}, nil
	}
	if found, ok := defaultBrowser(env); ok {
		return found, nil
	}
	for _, name := range env.Candidates() {
		if path, err := env.LookPath(name); err == nil {
			return Result{Path: path, Step: stepPath}, nil
		}
	}
	if found, ok := flatpakBrowser(env); ok {
		return found, nil
	}
	return Result{}, ErrNoBrowser
}

// defaultBrowser is the browser the user already chose, when it happens to be
// one lich can drive. It outranks the scan on purpose: a machine with three
// Chromium-family browsers installed has one the user actually uses, and lich
// picking a different one by list order is a window in the wrong browser with
// none of the user's extensions or window rules.
func defaultBrowser(env Env) (Result, bool) {
	for _, name := range defaultNames(env) {
		// A .desktop id is usually the executable plus that suffix
		// (vivaldi-stable.desktop); the ones where it is not are aliased, and
		// anything still unrecognised simply misses here, leaving the scan
		// below to answer.
		exe := strings.TrimSuffix(name, ".desktop")
		base := strings.ToLower(strings.TrimSuffix(filepath.Base(exe), ".exe"))
		if alias, ok := desktopExec[base]; ok {
			exe, base = alias, alias
		}
		if !knownChromium[base] {
			continue
		}
		if path, err := env.LookPath(exe); err == nil {
			return Result{Path: path, Step: stepDefault}, true
		}
	}
	return Result{}, false
}

// defaultNames lists what the desktop calls its default browser, best first:
// xdg-settings is the freedesktop answer, $BROWSER the older POSIX convention
// (a colon-separated list, whose entries may carry a %s for the URL — lich
// takes the command and passes its own arguments).
func defaultNames(env Env) []string {
	var names []string
	if xdg, err := env.LookPath("xdg-settings"); err == nil {
		if out, err := env.Output(xdg, "get", "default-web-browser"); err == nil {
			names = append(names, strings.TrimSpace(string(out)))
		}
	}
	for _, entry := range strings.Split(env.Getenv("BROWSER"), ":") {
		if fields := strings.Fields(entry); len(fields) > 0 {
			names = append(names, fields[0])
		}
	}
	return names
}

// flatpakBrowser is the last rung that still gives a real app window: on an
// immutable distribution the only Chromium on the machine is a Flatpak, and
// nothing about it is on PATH.
func flatpakBrowser(env Env) (Result, bool) {
	flatpak, err := env.LookPath("flatpak")
	if err != nil {
		return Result{}, false
	}
	// The profile has to land where the sandbox can write it, and HOME is what
	// names that directory. Without one the rung is skipped rather than
	// launched at a --user-data-dir the browser cannot create — which is a
	// window that never opens.
	home := env.Getenv("HOME")
	if home == "" {
		return Result{}, false
	}
	out, err := env.Output(flatpak, "list", "--app", "--columns=application")
	if err != nil {
		return Result{}, false
	}
	installed := map[string]bool{}
	for line := range strings.SplitSeq(string(out), "\n") {
		installed[strings.TrimSpace(line)] = true
	}
	for _, id := range flatpakIDs {
		if !installed[id] {
			continue
		}
		return Result{
			Path:        flatpak,
			Prefix:      []string{"run", id},
			Step:        stepFlatpak,
			ProfileRoot: filepath.Join(home, ".var", "app", id, "config"),
		}, true
	}
	return Result{}, false
}

// ParseFlags splits lich's own launch arguments: --browser <name-or-path> pins
// the browser to open the window in, and everything after `--` passes through
// to that browser. The flag wins over LICH_BROWSER because the caller writes it
// into the environment (main.go), which is also what carries it to the restart
// successor.
func ParseFlags(args []string) (pinned string, extra []string) {
	for i := 0; i < len(args); i++ {
		switch arg := args[i]; {
		case arg == "--":
			return pinned, args[i+1:]
		case arg == "--browser" && i+1 < len(args):
			i++
			pinned = args[i]
		case strings.HasPrefix(arg, "--browser="):
			pinned = strings.TrimPrefix(arg, "--browser=")
		}
	}
	return pinned, nil
}
