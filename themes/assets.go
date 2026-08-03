// Package themes owns the canonical bundled theme definitions.
package themes

import _ "embed"

var (
	//go:embed light.json
	Light []byte
	//go:embed dark.json
	Dark []byte
	// Template is the starter theme handed to a user who wants to write one.
	// It names every supported color, so it doubles as the shape reference.
	//go:embed template.json
	Template []byte
)
