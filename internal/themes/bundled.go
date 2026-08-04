package themes

import (
	"encoding/json"

	themeassets "github.com/omartelo/lich/themes"
)

func set(keys ...string) map[string]struct{} {
	out := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		out[key] = struct{}{}
	}
	return out
}

func keySet(values map[string]string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for key := range values {
		out[key] = struct{}{}
	}
	return out
}

func mustBundledTheme(data []byte) Theme {
	var theme Theme
	if err := json.Unmarshal(data, &theme); err != nil {
		panic("invalid bundled theme: " + err.Error())
	}
	// The constant is the invariant; the asset's own "origin" only exists
	// because the frontend imports the same JSON at build time. A typo there
	// would otherwise ship a theme the UI sorts and filters as neither
	// bundled nor custom.
	theme.Origin = OriginBundled
	return theme
}

var bundledThemes = []Theme{
	mustBundledTheme(themeassets.Light),
	mustBundledTheme(themeassets.Dark),
}

var appTokens = keySet(bundledThemes[0].App)

var terminalTokens = set(
	"background",
	"foreground",
	"cursor",
	"cursorAccent",
	"selectionBackground",
	"selectionForeground",
	"black",
	"red",
	"green",
	"yellow",
	"blue",
	"magenta",
	"cyan",
	"white",
	"brightBlack",
	"brightRed",
	"brightGreen",
	"brightYellow",
	"brightBlue",
	"brightMagenta",
	"brightCyan",
	"brightWhite",
)
