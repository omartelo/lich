//go:build !windows && !darwin

package chromium

// browserCandidates lists candidate binaries in preference order. Any
// Chromium gives the same compositor; the preference only picks the most
// conventional install. All bare names — Linux/BSD installs live on PATH.
// macOS keeps browsers in .app bundles off PATH, so it has its own list
// (candidates_darwin.go).
func browserCandidates() []string {
	return []string{
		"chromium",
		"chromium-browser",
		"google-chrome-stable",
		"google-chrome",
		"helium-browser",
		"brave",
	}
}
