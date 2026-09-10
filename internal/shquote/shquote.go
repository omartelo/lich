// Package shquote quotes a string for a shell command line. lich composes
// three of those — the worktree setup wrapper that execs a provider
// (internal/terminal) and the terminal-editor invocation handed to a session
// (internal/system), on a POSIX shell; and the same editor invocation on
// Windows, where the session's shell is PowerShell. All of them interpolate a
// path or a binary name the user or an agent chose. One copy of each rule, so a
// metacharacter that survives one call site cannot survive the other.
//
// The window composes a fourth — the `lich send` line a session's card hands
// out — and carries both rules the same way in
// frontend/src/lib/session/send-command.ts, the page-side half of this file,
// tested against the same cases. It picks between them by the platform the
// window itself runs on: that line is pasted into a terminal on this machine,
// so the shell that decides is the user's own.
package shquote

import "strings"

// Quote returns s single-quoted, safe against embedded single quotes. It
// assumes an sh/bash/zsh-style shell; PowerShell, which a Windows session runs,
// spells the escape differently — see QuotePwsh.
func Quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// QuotePwsh returns s single-quoted for PowerShell, safe against embedded
// single quotes. PowerShell expands nothing inside a single-quoted string — no
// $variable, no `backtick escape, no subexpression — so doubling the quote is
// the whole rule, and unlike cmd.exe (which lich no longer opens a session in)
// there is no character it cannot express.
func QuotePwsh(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
