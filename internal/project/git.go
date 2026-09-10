package project

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/omartelo/lich/internal/winexec"
)

// waitDelay caps how long a call keeps this process after the child itself is
// gone. A pipe reaches EOF only when its last writer closes it, so a grandchild
// that inherited the child's stdout keeps Wait blocked long after the child and
// its deadline are gone: the context kills the child it named and the call waits
// on regardless — a budget of 200ms measured at ten seconds, reporting success.
// WaitDelay ends that wait: the pipes are closed and Wait reports
// exec.ErrWaitDelay.
//
// Nothing here is known to do it. The three that come to mind — an ssh
// ControlMaster left alive by a fetch, git's credential cache daemon, the
// gc --auto git detaches — were each measured still running after their call
// returned at once, because openssh and git both hand back stdio before they
// detach. This holds the budget against one that does not.
//
// Two seconds is generous because the clock only starts once the child is dead,
// where draining what is left in a pipe takes microseconds.
const waitDelay = 2 * time.Second

// noPromptEnv is what keeps a child from stopping on a question nobody will
// answer. Started from a terminal, lich inherits it and hands it to every child
// here, so a git that stops to ask for a username asks *there* — behind the
// window, where the answer never comes: measured, git waits at that prompt until
// it is killed, and fails on its own in a second and a half with this set. The
// calls that take no context have no budget to end the wait for them.
//
// The terminal prompt alone: a credential helper and an askpass program still
// answer, and those are how a machine set up for this answers with nobody at the
// keyboard.
var noPromptEnv = []string{"GIT_TERMINAL_PROMPT=0", "GH_PROMPT_DISABLED=1"}

// command builds an exec.Cmd for a child console tool (git, gh). Used across
// this package instead of exec.Command so no call site can forget what prepare
// applies.
func command(name string, args ...string) *exec.Cmd {
	return prepare(exec.Command(name, args...))
}

// commandContext is command for a call that must be killed if it outlives ctx —
// anything reaching the network, where "slow" has no upper bound.
func commandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	return prepare(exec.CommandContext(ctx, name, args...))
}

// prepare applies what every child of this package gets: no console window on
// Windows, which lich's GUI-subsystem binary would otherwise pop one of per
// spawn (winexec.Hide); no wait on a grandchild (waitDelay); no prompt
// (noPromptEnv). A call site with a variable of its own appends to cmd.Env —
// assigning over it drops these.
func prepare(cmd *exec.Cmd) *exec.Cmd {
	winexec.Hide(cmd)
	cmd.WaitDelay = waitDelay
	cmd.Env = append(os.Environ(), noPromptEnv...)
	return cmd
}

// runGit runs git -C dir args... for a call whose failure is shown to someone:
// it comes back as the screen's message and git's stderr reaches the log
// (giterror.go). Every failure it sees is reported as one.
//
// That makes it the wrong runner for two other kinds of call, which take
// gitQuiet instead: a polled read, where a failure means "no answer, try again
// in a second", and a probe whose negative answer *is* the result ("does HEAD
// know this file?"). Routed here, both would file a warning per call — several
// per second on a repository with no commits — and bury the failures that
// actually mean something.
func runGit(dir string, args ...string) (string, error) {
	cmd := command("git", append([]string{"-C", dir}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", gitFailure(args, stderr.String(), err)
	}
	return string(out), nil
}

// gitQuiet runs git -C dir args... and reports only whether it succeeded. ok is
// false for every failure alike — the caller asked a question git may legitimately
// answer "no" to, so there is nothing to log and nothing to show. See runGit for
// which calls belong here and which do not.
func gitQuiet(dir string, args ...string) (string, bool) {
	out, err := command("git", append([]string{"-C", dir}, args...)...).Output()
	if err != nil {
		return "", false
	}
	return string(out), true
}

// GitCommonDir returns the repository metadata directory shared by a checkout
// and every worktree linked to it, or "" when dir is not in a repository. It is
// the same question basestatus.go asks, in the same spelling, and absolute
// because the caller needs a path rather than something to resolve against dir.
//
// A linked worktree's `.git` is a file naming that directory, so a process that
// can see the worktree and not the common directory has a checkout git refuses
// to read at all. It is asked of git rather than parsed out of the `.git` file
// because git is the one thing that cannot be wrong about its own layout.
func GitCommonDir(dir string) string {
	out, ok := gitQuiet(dir, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if !ok {
		return ""
	}
	return strings.TrimSpace(out)
}

// splitNUL splits the -z output of a git list command into its entries. git
// terminates each with a NUL, so the trailing empty element is dropped along
// with any other.
func splitNUL(out string) []string {
	var entries []string
	for entry := range strings.SplitSeq(out, "\x00") {
		if entry != "" {
			entries = append(entries, entry)
		}
	}
	return entries
}

// splitLines splits command output into its non-empty lines.
func splitLines(out string) []string {
	lines := []string{}
	for line := range strings.Lines(out) {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return lines
}
