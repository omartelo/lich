//go:build !windows && !darwin

package chromium

import (
	"slices"
	"testing"
)

// TestUnixBrowserCandidates pins the exact Linux/BSD candidate list in
// preference order: bare PATH names (Linux/BSD installs live on PATH), Chromium
// first. It mirrors the exact-list style of TestWindowsBrowserCandidates /
// TestDarwinBrowserCandidates — a membership check would miss an order
// regression or a stray entry — and it spells the list out rather than reading
// chromiumNames, which is the thing under test.
//
// The build tag matches candidates_unix.go (`!windows && !darwin`) on purpose:
// under a bare `!windows` this test also compiles into the darwin build, where
// browserCandidates() returns the .app list and none of these bare names, so
// the assertion would fail the macOS release suite.
func TestUnixBrowserCandidates(t *testing.T) {
	want := []string{
		"chromium",
		"chromium-browser",
		"google-chrome",
		"google-chrome-stable",
		"google-chrome-beta",
		"helium-browser",
		"brave",
		"brave-browser",
		"vivaldi",
		"vivaldi-stable",
		"microsoft-edge",
		"microsoft-edge-stable",
		"thorium-browser",
		"ungoogled-chromium",
		"chrome",
	}
	if got := browserCandidates(); !slices.Equal(got, want) {
		t.Fatalf("browserCandidates() = %v, want %v", got, want)
	}
}
