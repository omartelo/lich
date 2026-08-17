package terminal

import (
	"strings"

	"github.com/omartelo/lich/internal/shquote"
)

// wrapEntrypoint rewrites a terminal session's spawn so its PTY opens straight
// into one command — lazygit, k9s, `pnpm dev` — instead of a bare prompt. The
// command is stored on the session row, so it comes back on every later spawn:
// the card's first view, a respawn after a reload, the resume of a parked
// worktree session.
//
// It borrows wrapSetup's idiom, and shares nothing else with it. The setup
// script belongs to the project, is written into the repository, runs once when
// a worktree is born, and runs for whatever kind of session created it. This
// belongs to one row, is typed into a dialog on one card, runs on every spawn of
// that card, and reaches only a shell. The kind check lives in here rather than
// at the call site for that reason: one place decides, so there is no second
// caller to forget it. The write side is guarded independently, in SQL
// (store.SetSessionEntrypoint) — neither guard depends on the other holding.
//
// The command exits into the shell rather than taking the PTY with it. A card
// whose process is gone is a card the user has to notice and revive, which is
// the wrong answer for both halves of what this is for: quitting lazygit means
// wanting a prompt, and a dev server that crashed wants its error still on
// screen with a shell under it.
//
// goos is runtime.GOOS, passed in so the decision stays pure and testable
// off-Windows — wrapSetup's pattern, and Windows is skipped for wrapSetup's
// reason: composing a cmd.exe chain is not worth it while the port stays
// experimental. shell is the resolved user shell, which is also what the command
// is run by: a value the user typed is their own shell's syntax, not sh's.
func wrapEntrypoint(spec ptySpec, kind, entrypoint, goos string) ptySpec {
	entrypoint = strings.TrimSpace(entrypoint)
	if kind != KindShell || entrypoint == "" || goos == "windows" {
		return spec
	}
	// The newline is load-bearing: a command ending in a comment or an unclosed
	// `&&` would otherwise swallow the exec that follows it.
	spec.args = []string{"-c", entrypoint + "\nexec " + shquote.Quote(spec.bin)}
	return spec
}
