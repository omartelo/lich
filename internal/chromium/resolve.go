package chromium

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ErrNoShell is an install with no window of its own: nothing at shellPaths
// and no pin. It is a sentinel because the caller answers it differently from
// every other failure: on macOS lich still runs, in a plain tab of the default
// browser (main.go's openWithoutWindow), while everywhere else it is the error
// the user has to see — a package missing its window, or a bare binary.
var ErrNoShell = errors.New(
	"no window beside the lich binary — lich-shell is missing; install lich from a package, " +
		"or point LICH_SHELL at a window build")

// OverrideEnv pins the window to launch, by path. The --shell flag is carried
// in it rather than passed down as an argument, so the restart successor
// inherits the choice and `lich doctor` reports the window a launch here would
// actually open. `task dev` is the one caller: `go run` has no window beside
// its binary.
const OverrideEnv = "LICH_SHELL"

// The rung of the ladder that answered, as the diagnostics name it.
const stepPinned = "pinned by LICH_SHELL"

// Env is everything resolution reads from the machine. Every probe is a field
// so the ladder is table-testable on a machine with no window built.
type Env struct {
	LookPath func(name string) (string, error)
	Getenv   func(key string) string
	// Shell is the path of the window this install bundles, "" when it has
	// none (shell.go).
	Shell func() string
}

// RealEnv reaches the machine lich is running on.
func RealEnv() Env {
	return Env{
		LookPath: exec.LookPath,
		Getenv:   os.Getenv,
		Shell:    bundledShell,
	}
}

// Result is a resolved launch: which executable, and which rung named it.
type Result struct {
	// Path is the executable to run.
	Path string
	// Step names the rung that answered, for `lich doctor` and `lich rage`.
	Step string
}

// Describe is the resolution as a diagnostic prints it: the command, and the
// rung that produced it.
func (r Result) Describe() string {
	return fmt.Sprintf("%s (%s)", r.Path, r.Step)
}

// Resolve finds the window: the one the user pinned, else the one lich itself
// bundles. It returns ErrNoShell when the install has neither — see that
// variable for what the caller makes of it.
func Resolve(env Env) (Result, error) {
	if pinned := strings.TrimSpace(env.Getenv(OverrideEnv)); pinned != "" {
		path, err := env.LookPath(pinned)
		if err != nil {
			// Loud, and never a fall-through to the bundled window: the user
			// named this one, and quietly opening a different one is the whole
			// class of bug the override exists to rule out.
			return Result{}, fmt.Errorf("%s=%s: %w", OverrideEnv, pinned, err)
		}
		return Result{Path: path, Step: stepPinned}, nil
	}
	if shell := env.Shell(); shell != "" {
		return Result{Path: shell, Step: stepShell}, nil
	}
	return Result{}, ErrNoShell
}

// ParseFlags splits lich's own launch arguments: --shell <path> pins the
// window to open, and everything after `--` passes through to it as Chromium
// switches. The flag wins over its environment variable because the caller
// writes it into the environment (main.go), which is also what carries it to
// the restart successor.
func ParseFlags(args []string) (pinned string, extra []string) {
	for i := 0; i < len(args); i++ {
		switch arg := args[i]; {
		case arg == "--":
			return pinned, args[i+1:]
		case arg == "--shell" && i+1 < len(args):
			i++
			pinned = args[i]
		case strings.HasPrefix(arg, "--shell="):
			pinned = strings.TrimPrefix(arg, "--shell=")
		}
	}
	return pinned, nil
}
