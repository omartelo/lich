package providers

import (
	"os"
	"path/filepath"
)

// CursorConfigDir resolves Cursor CLI's config directory under home — where it
// keeps the credentials (`auth.json`, `cli-config.json`) and the chats.
//
// It lives here because two packages need the same answer and got two: the
// sandbox binds this directory into a confined session, and the terminal reads
// the chats under it to decide whether a conversation can still be resumed. A
// second copy of the rule had already drifted on whether a relative variable
// counts.
//
// Cursor is not xdg-basedir, which is the trap: $CURSOR_CONFIG_DIR wins
// outright, else $XDG_CONFIG_HOME with "cursor" under it — and with neither set
// it lands on ~/.cursor, never ~/.config/cursor. Reading it the way opencode and
// Crush are read would name a directory the CLI never writes on a machine with
// no XDG_CONFIG_HOME, which is most of them. Measured on 2026.08.11.
//
// A relative variable is ignored rather than resolved, both because
// XDG_CONFIG_HOME is specified that way and because there is nothing to resolve
// it against: lich's working directory is not the CLI's, so a relative path here
// names a directory nobody involved would agree on.
//
// It answers for the config dir alone. `~/.cursor` is a second directory the CLI
// resolves off the home with no variable in the way at all — it holds mcp.json,
// the per-project transcripts and the CLI state — and a caller that needs the
// whole picture binds both (internal/sandbox).
func CursorConfigDir(home string) string {
	if dir := os.Getenv("CURSOR_CONFIG_DIR"); filepath.IsAbs(dir) {
		return dir
	}
	if base := os.Getenv("XDG_CONFIG_HOME"); filepath.IsAbs(base) {
		return filepath.Join(base, "cursor")
	}
	return filepath.Join(home, ".cursor")
}
