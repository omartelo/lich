package project

import (
	"bytes"
	"context"
	"os/exec"
	"strings"

	"github.com/omartelo/lich/internal/winexec"
)

// command builds an exec.Cmd for a child console tool (git, gh) with its
// console window suppressed on Windows — lich's GUI-subsystem binary would
// otherwise pop one per spawn (winexec.Hide). Used across this package instead
// of exec.Command so no call site can forget it.
func command(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	winexec.Hide(cmd)
	return cmd
}

// commandContext is command for a call that must be killed if it outlives ctx —
// anything reaching the network, where "slow" has no upper bound.
func commandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	winexec.Hide(cmd)
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
