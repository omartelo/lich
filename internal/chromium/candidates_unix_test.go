//go:build !windows

package chromium

import (
	"slices"
	"testing"
)

func TestUnixBrowserCandidatesIncludeSupportedChromiumFamilyBrowsers(t *testing.T) {
	candidates := browserCandidates()
	for _, want := range []string{
		"chromium",
		"chromium-browser",
		"google-chrome-stable",
		"google-chrome",
		"helium-browser",
		"brave",
	} {
		if !slices.Contains(candidates, want) {
			t.Fatalf("browserCandidates() = %v, missing %q", candidates, want)
		}
	}
}
