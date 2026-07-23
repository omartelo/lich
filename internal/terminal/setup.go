package terminal

import "strings"

// wrapSetup rewrites a session's spawn so the project's worktree setup script
// runs first, in the same PTY — its output lands in the terminal the user is
// already looking at. The script runs in a subshell (a cd inside it cannot
// move the provider's start directory) and the provider starts even when the
// script fails: a broken setup must not cost the session, so the failure is
// echoed and the provider execs anyway. goos is runtime.GOOS, passed in so the
// decision stays pure and testable off-Windows (wrapArgv's pattern): Windows
// is skipped — composing a cmd.exe chain around wrapArgv's own cmd.exe
// handling is not worth it while the port stays experimental.
func wrapSetup(spec ptySpec, script, goos string) ptySpec {
	if script == "" || goos == "windows" {
		return spec
	}
	argv := make([]string, 0, len(spec.args)+1)
	for _, arg := range append([]string{spec.bin}, spec.args...) {
		argv = append(argv, shQuote(arg))
	}
	spec.bin = "sh"
	spec.args = []string{
		"-c",
		"(\n" + script + "\n) || echo \"[lich] worktree setup failed (exit $?)\"; exec " + strings.Join(argv, " "),
	}
	return spec
}

// shQuote returns s single-quoted for POSIX sh, safe against embedded quotes.
func shQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
