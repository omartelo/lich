//go:build windows

package chromium

import "os"

// browserCandidates lists candidate browsers in preference order. Windows
// installs don't land on PATH, so the list is mostly absolute paths built
// from the install-root environment variables (windowsBrowserCandidates in
// chromium.go, where the logic stays testable from any OS).
func browserCandidates() []string {
	return windowsBrowserCandidates(os.Getenv)
}
