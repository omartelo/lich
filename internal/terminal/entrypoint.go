package terminal

import (
	"encoding/base64"
	"encoding/binary"
	"strings"
	"unicode/utf16"

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
// off-Windows — wrapSetup's pattern. shell is the resolved user shell, which is
// also what the command is run by: a value the user typed is their own shell's
// syntax, not sh's. Which is why the two branches share no composition. A POSIX
// shell reaches the prompt by exec'ing itself over the command, and PowerShell —
// what a Windows session runs (windowsShells) — has -NoExit for exactly this,
// which is the better half of the trade: the shell the user lands in is the same
// process lich spawned, so the pid it polls for the working directory is still
// the shell's (cwd.go) and no second process hangs off the PTY.
func wrapEntrypoint(spec ptySpec, kind, entrypoint, goos string) ptySpec {
	entrypoint = strings.TrimSpace(entrypoint)
	if kind != KindShell || entrypoint == "" {
		return spec
	}
	if goos == "windows" {
		// No -NoProfile: the profile is the user's own rc, this is their own
		// shell, and PowerShell loads it before running the command — so an
		// alias defined in $PROFILE can be an entrypoint, the one thing the
		// POSIX branch cannot offer.
		spec.args = []string{"-NoExit", "-EncodedCommand", encodePwshCommand(entrypoint)}
		return spec
	}
	// The newline is load-bearing: a command ending in a comment or an unclosed
	// `&&` would otherwise swallow the exec that follows it.
	spec.args = []string{"-c", entrypoint + "\nexec " + shquote.Quote(spec.bin)}
	return spec
}

// encodePwshCommand renders a script as the base64 of its UTF-16LE bytes, which
// is what PowerShell's -EncodedCommand takes.
//
// The encoding is here to sidestep quoting, not to hide anything. A Windows
// command line is one string: the argv lich builds is composed by
// windows.ComposeCommandLine (pty_windows.go), which escapes an embedded double
// quote as \" for CommandLineToArgvW — and PowerShell does not read \" as an
// escape. An entrypoint is text a user typed into a dialog, so a double quote in
// it (`pnpm run "dev server"`) is ordinary rather than hostile, and it would
// arrive mangled or split. Base64 has no character either parser acts on, so
// there is no rule left to get wrong.
func encodePwshCommand(script string) string {
	units := utf16.Encode([]rune(script))
	buf := make([]byte, 2*len(units))
	for i, unit := range units {
		binary.LittleEndian.PutUint16(buf[2*i:], unit)
	}
	return base64.StdEncoding.EncodeToString(buf)
}
