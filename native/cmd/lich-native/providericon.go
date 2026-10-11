package main

import "github.com/omartelo/lich/native/ui/icons"

// providerIcons is ProviderIcon.tsx: the Lobe mark of each provider, oh-my-pi
// wearing Pi's, and a Lucide glyph for the kinds Lobe has no mark for. Its
// keys are every session kind (sessions.ts isSessionKind).
var providerIcons = map[string]*icons.Icon{
	"claude":      icons.Lobe("claude"),
	"codex":       icons.Lobe("codex"),
	"antigravity": icons.Lobe("antigravity"),
	"opencode":    icons.Lobe("opencode"),
	"omp":         icons.Lobe("pi"),
	"cursor":      icons.Lobe("cursor"),
	"crush":       icons.Lucide("sparkles"),
	"kiro":        icons.Lucide("ghost"),
	"shell":       icons.Lucide("terminal"),
}

// providerIcon falls back to the terminal glyph for a kind from a newer
// backend, as ProviderIcon does.
func providerIcon(kind string) *icons.Icon {
	if ic, ok := providerIcons[kind]; ok {
		return ic
	}
	return providerIcons["shell"]
}
