package project

import (
	"errors"
	"log/slog"
	"os/exec"
	"strings"
)

// lich drives two command-line tools, gh and git, and both write their failures
// for a terminal: "fatal:" prefixes, GraphQL envelopes, flag hints, exit
// statuses. None of that belongs on screen — so each tool keeps a table of the
// failures lich can actually hit (gherror.go, giterror.go), the raw text goes to
// the log, and the screen gets a sentence it can act on.
//
// The messages are causes, not sentences about an operation: every call site in
// the frontend already says which one failed ("Failed to remove worktree: …").

// toolFailure is one recognised failure: what the tool's stderr contains,
// lowercased, and what the screen says in its place.
type toolFailure struct{ match, message string }

// matchFailure returns the message for the first entry the stderr matches, so
// the specific entries in a table have to come before the general ones.
func matchFailure(failures []toolFailure, stderr string) (string, bool) {
	low := strings.ToLower(stderr)
	for _, failure := range failures {
		if strings.Contains(low, failure.match) {
			return failure.message, true
		}
	}
	return "", false
}

// missingTool reports the failure of a tool that is not installed at all — the
// one failure exec reports instead of the tool.
func missingTool(stderr string, runErr error) bool {
	return errors.Is(runErr, exec.ErrNotFound) ||
		strings.Contains(strings.ToLower(stderr), "executable file not found")
}

// logToolFailure keeps what the tool actually said, which is the part worth
// reading when a message here turns out to be the wrong guess.
func logToolFailure(tool string, args []string, stderr string, runErr error) {
	slog.Warn(tool+" failed",
		"command", strings.Join(args, " "),
		"err", runErr,
		"stderr", strings.TrimSpace(stderr))
}
