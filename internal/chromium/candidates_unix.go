//go:build !windows

package chromium

// browserCandidates lists candidate binaries in preference order. Any
// Chromium gives the same compositor; the preference only picks the most
// conventional install. All bare names — Unix installs live on PATH.
func browserCandidates() []string {
	return []string{
		"chromium",
		"chromium-browser",
		"google-chrome-stable",
		"google-chrome",
		"brave",
	}
}
