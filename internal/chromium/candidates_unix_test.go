//go:build !windows && !darwin

package chromium

import (
	"slices"
	"testing"
)

// TestUnixBrowserCandidates pins the exact Linux/BSD candidate list in
// preference order: bare PATH names (Linux/BSD installs live on PATH), Chromium
// first, Helium probed before Brave. It mirrors the exact-list style of
// TestWindowsBrowserCandidates / TestDarwinBrowserCandidates — a membership
// check would miss an order regression or a stray entry.
//
// The build tag matches candidates_unix.go (`!windows && !darwin`) on purpose:
// under a bare `!windows` this test also compiles into the darwin build, where
// browserCandidates() returns the .app list and none of these bare names, so
// the assertion would fail the macOS release suite.
func TestUnixBrowserCandidates(t *testing.T) {
	want := []string{
		"chromium",
		"chromium-browser",
		"google-chrome-stable",
		"google-chrome",
		"helium-browser",
		"brave",
	}
	if got := browserCandidates(); !slices.Equal(got, want) {
		t.Fatalf("browserCandidates() = %v, want %v", got, want)
	}
}
