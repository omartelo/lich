// Package themes owns the canonical bundled theme definitions.
package themes

import _ "embed"

var (
	//go:embed light.json
	Light []byte
	//go:embed dark.json
	Dark []byte
)
