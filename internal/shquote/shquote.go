// Package shquote quotes a string for a POSIX shell command line. lich composes
// two of those — the worktree setup wrapper that execs a provider
// (internal/terminal) and the terminal-editor invocation handed to a session
// (internal/system) — and both interpolate a path or a binary name the user or
// an agent chose. One copy of the rule, so a metacharacter that survives one
// call site cannot survive the other.
package shquote

import "strings"

// Quote returns s single-quoted, safe against embedded single quotes. It
// assumes an sh/bash/zsh-style shell; cmd.exe (experimental Windows) quotes
// differently, which is why the setup wrapper skips Windows entirely.
func Quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
