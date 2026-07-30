package themes

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

var bundledThemes = []Theme{
	{
		ID:     "light",
		Name:   "Light",
		Scheme: SchemeLight,
		Origin: OriginBundled,
		App: map[string]string{
			"background":                 "oklch(0.94 0.003 286.32)",
			"foreground":                 "oklch(0.21 0.006 285.885)",
			"card":                       "oklch(0.98 0.002 286.32)",
			"card-foreground":            "oklch(0.21 0.006 285.885)",
			"popover":                    "oklch(0.98 0.002 286.32)",
			"popover-foreground":         "oklch(0.21 0.006 285.885)",
			"primary":                    "oklch(0.21 0.006 285.885)",
			"primary-foreground":         "oklch(0.985 0 0)",
			"secondary":                  "oklch(0.92 0.003 286.32)",
			"secondary-foreground":       "oklch(0.21 0.006 285.885)",
			"muted":                      "oklch(0.935 0.003 286.32)",
			"muted-foreground":           "oklch(0.505 0.016 285.938)",
			"accent":                     "oklch(0.9 0.004 286.32)",
			"accent-foreground":          "oklch(0.21 0.006 285.885)",
			"destructive":                "oklch(0.577 0.245 27.325)",
			"border":                     "oklch(0.888 0.004 286.32)",
			"input":                      "oklch(0.888 0.004 286.32)",
			"ring":                       "oklch(0.6 0.015 286.067)",
			"chart-1":                    "oklch(0.871 0.006 286.286)",
			"chart-2":                    "oklch(0.552 0.016 285.938)",
			"chart-3":                    "oklch(0.442 0.017 285.786)",
			"chart-4":                    "oklch(0.37 0.013 285.805)",
			"chart-5":                    "oklch(0.274 0.006 286.033)",
			"sidebar":                    "oklch(0.98 0.002 286.32)",
			"sidebar-foreground":         "oklch(0.21 0.006 285.885)",
			"sidebar-primary":            "oklch(0.21 0.006 285.885)",
			"sidebar-primary-foreground": "oklch(0.985 0 0)",
			"sidebar-accent":             "oklch(0.9 0.004 286.32)",
			"sidebar-accent-foreground":  "oklch(0.21 0.006 285.885)",
			"sidebar-border":             "oklch(0.888 0.004 286.32)",
			"sidebar-ring":               "oklch(0.6 0.015 286.067)",
		},
		Terminal: map[string]string{
			"background": "#e8e8ea",
			"foreground": "#1f2328",
		},
	},
	{
		ID:     "dark",
		Name:   "Dark",
		Scheme: SchemeDark,
		Origin: OriginBundled,
		App: map[string]string{
			"background":                 "oklch(0.141 0.005 285.823)",
			"foreground":                 "oklch(0.985 0 0)",
			"card":                       "oklch(0.21 0.006 285.885)",
			"card-foreground":            "oklch(0.985 0 0)",
			"popover":                    "oklch(0.21 0.006 285.885)",
			"popover-foreground":         "oklch(0.985 0 0)",
			"primary":                    "oklch(0.92 0.004 286.32)",
			"primary-foreground":         "oklch(0.21 0.006 285.885)",
			"secondary":                  "oklch(0.274 0.006 286.033)",
			"secondary-foreground":       "oklch(0.985 0 0)",
			"muted":                      "oklch(0.274 0.006 286.033)",
			"muted-foreground":           "oklch(0.705 0.015 286.067)",
			"accent":                     "oklch(0.274 0.006 286.033)",
			"accent-foreground":          "oklch(0.985 0 0)",
			"destructive":                "oklch(0.704 0.191 22.216)",
			"border":                     "oklch(1 0 0 / 10%)",
			"input":                      "oklch(1 0 0 / 15%)",
			"ring":                       "oklch(0.552 0.016 285.938)",
			"chart-1":                    "oklch(0.871 0.006 286.286)",
			"chart-2":                    "oklch(0.552 0.016 285.938)",
			"chart-3":                    "oklch(0.442 0.017 285.786)",
			"chart-4":                    "oklch(0.37 0.013 285.805)",
			"chart-5":                    "oklch(0.274 0.006 286.033)",
			"sidebar":                    "oklch(0.21 0.006 285.885)",
			"sidebar-foreground":         "oklch(0.985 0 0)",
			"sidebar-primary":            "oklch(0.488 0.243 264.376)",
			"sidebar-primary-foreground": "oklch(0.985 0 0)",
			"sidebar-accent":             "oklch(0.274 0.006 286.033)",
			"sidebar-accent-foreground":  "oklch(0.985 0 0)",
			"sidebar-border":             "oklch(1 0 0 / 10%)",
			"sidebar-ring":               "oklch(0.552 0.016 285.938)",
		},
		Terminal: map[string]string{
			"background": "#06070f",
			"foreground": "#e5e7eb",
		},
	},
}
