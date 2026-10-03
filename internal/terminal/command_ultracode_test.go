package terminal

import (
	"slices"
	"testing"

	"github.com/omartelo/lich/internal/providers"
)

func TestUltracodeArgs(t *testing.T) {
	if got := ultracodeArgs(providers.Claude, true); !slices.Equal(got, []string{"--settings", `{"ultracode":true}`}) {
		t.Errorf("claude, on: args = %v, want --settings with the ultracode key", got)
	}
	if got := ultracodeArgs(providers.Claude, false); got != nil {
		t.Errorf("claude, off: args = %v, want none", got)
	}
	for _, kind := range []string{
		providers.Codex, providers.Antigravity, providers.OpenCode, providers.OMP,
		providers.Crush, providers.Cursor, providers.Kiro, KindShell,
	} {
		if got := ultracodeArgs(kind, true); got != nil {
			t.Errorf("%s, on: args = %v, want none — only Claude Code has an ultracode", kind, got)
		}
		if SupportsUltracode(kind) {
			t.Errorf("SupportsUltracode(%q) = true, want false", kind)
		}
	}
}
