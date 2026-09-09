package terminal

import (
	"strings"

	"github.com/omartelo/lich/internal/project"
	"github.com/omartelo/lich/internal/shquote"
)

// wrapSetup rewrites a session's spawn so the project's worktree setup script
// runs first, in the same PTY — its output lands in the terminal the user is
// already looking at. The script runs in a subshell (a cd inside it cannot
// move the provider's start directory) and the provider starts even when the
// script fails: a broken setup must not cost the session, so the failure is
// echoed and the provider execs anyway. goos is runtime.GOOS, passed in so the
// decision stays pure and testable off-Windows (wrapArgv's pattern): Windows is
// skipped, and not for the shell's sake — a Windows session runs PowerShell now
// (windowsShells), which wrapEntrypoint composes for. What is skipped is the
// script: project.SetupScriptPath is `.lich/setup-worktree.sh`, one file, versioned in
// the repository and shared by everyone who checks it out, and its contents are
// sh. Running that through PowerShell would not fail cleanly — it would run the
// leading words of every line as commands.
//
// The bool reports whether the wrap happened, and it is the only honest answer
// to "will this PTY print the end marker". Re-deriving it from the returned
// spec reads sh — which a provider's own binary may already be — and a session
// armed to wait for a marker nothing will print never becomes ready.
func wrapSetup(spec ptySpec, script, goos string) (ptySpec, bool) {
	if script == "" || goos == "windows" {
		return spec, false
	}
	argv := make([]string, 0, len(spec.args)+1)
	for _, arg := range append([]string{spec.bin}, spec.args...) {
		argv = append(argv, shquote.Quote(arg))
	}
	spec.bin = "sh"
	spec.args = []string{
		"-c",
		"(\n" + script + "\n) || echo \"[lich] worktree setup failed (exit $?)\"; " +
			"printf '" + setupDoneEscaped + "'; exec " + strings.Join(argv, " "),
	}
	return spec, true
}

// setupDone is what the wrapper emits between the script and the provider, and
// the only way lich can tell the two apart from outside: the PTY is the same,
// the process is the same (exec replaces the image, keeping the pid), and the
// output of a setup script looks like the output of anything else.
//
// It matters because a session running its setup is not a session an agent can
// be given work in. lich types a relayed message at the prompt, and during the
// setup the thing reading that PTY is the script — a message handed over then
// goes to `pnpm install`, which discards it, and the agent that finally starts
// never sees the request. Nothing reports a failure: the sender waits for an
// answer nobody was ever asked for.
//
// An OSC sequence with a private code carries it. Terminals ignore an OSC they
// do not know — xterm.js parses and drops it — so the marker never reaches the
// screen the user is watching.
const (
	setupDone        = "\x1b]6969;lich-setup-done\x07"
	setupDoneEscaped = "\\033]6969;lich-setup-done\\007"
)

// setupSkippedNotice is the line a Windows session prints in its card, and ""
// wherever there is nothing to say. It answers the silence wrapSetup used to
// leave: the checkout opens with its dependencies not installed, and the first
// thing that fails is the agent's own command, for a reason only this file
// knows.
//
// goos is wrapSetup's seam, and the two decide from the same pair — a script
// wrapSetup refuses is exactly a script this announces.
//
// It is written into the session's output (Service.spawnSession) rather than
// composed into the spawn, because there is no exec on Windows: printing the
// line from PowerShell would leave PowerShell between lich and the provider for
// the life of the session, and the pid the cwd poller reads is the one lich
// spawned.
func setupSkippedNotice(script, goos string) string {
	if script == "" || goos != "windows" {
		return ""
	}
	// CRLF because a PTY's own newline is one: the card would otherwise keep
	// the column the line ended in and let the provider draw from there.
	return "[lich] setup skipped: " + project.SetupScriptPath +
		" is sh and this session runs PowerShell.\r\n"
}
