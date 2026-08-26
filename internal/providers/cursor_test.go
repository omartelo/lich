package providers

import (
	"path/filepath"
	"testing"
)

// The precedence Cursor was measured to apply, and the one thing about it that
// surprises: with neither variable set it lands on ~/.cursor, not on the
// xdg-basedir ~/.config/cursor every other provider here follows.
func TestCursorConfigDirPrecedence(t *testing.T) {
	home := t.TempDir()
	explicit := t.TempDir()
	xdg := t.TempDir()

	t.Setenv("CURSOR_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	if got, want := CursorConfigDir(home), filepath.Join(home, ".cursor"); got != want {
		t.Errorf("with nothing set = %q, want %q", got, want)
	}

	t.Setenv("XDG_CONFIG_HOME", xdg)
	if got, want := CursorConfigDir(home), filepath.Join(xdg, "cursor"); got != want {
		t.Errorf("with XDG_CONFIG_HOME = %q, want %q", got, want)
	}

	t.Setenv("CURSOR_CONFIG_DIR", explicit)
	if got := CursorConfigDir(home); got != explicit {
		t.Errorf("CURSOR_CONFIG_DIR loses to XDG_CONFIG_HOME: %q, want %q", got, explicit)
	}
}

// A relative variable is ignored, not resolved. This is the rule the two copies
// of this resolver disagreed on: one required an absolute path and the other
// took whatever was set, so a relative CURSOR_CONFIG_DIR had the sandbox binding
// ~/.cursor while the resume check looked somewhere else entirely.
func TestCursorConfigDirIgnoresARelativeVariable(t *testing.T) {
	home := t.TempDir()
	want := filepath.Join(home, ".cursor")

	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("CURSOR_CONFIG_DIR", filepath.Join("relative", "cursor"))
	if got := CursorConfigDir(home); got != want {
		t.Errorf("a relative CURSOR_CONFIG_DIR = %q, want %q", got, want)
	}

	t.Setenv("CURSOR_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", filepath.Join("relative", "config"))
	if got := CursorConfigDir(home); got != want {
		t.Errorf("a relative XDG_CONFIG_HOME = %q, want %q", got, want)
	}
}
