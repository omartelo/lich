//go:build darwin

package chromium

import "os"

// browserCandidates lists candidate browsers in preference order. macOS
// installs land in .app bundles under /Applications, not on PATH, so the list
// is absolute paths built from the install roots (darwinBrowserCandidates in
// chromium.go, where the logic stays testable from any OS).
func browserCandidates() []string {
	return darwinBrowserCandidates(os.Getenv)
}
