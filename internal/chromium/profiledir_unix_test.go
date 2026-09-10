//go:build !windows

package chromium

import "testing"

// TestProfileKeyIsTheOneOnDisk pins the digest to the one every profile was
// keyed by before the window was the only launch: a change here hands every
// user a factory-fresh profile on update. Unix only because filepath.Clean
// turns the slashes into backslashes on Windows, where the key on disk is the
// one of the backslash path the installer lays down.
func TestProfileKeyIsTheOneOnDisk(t *testing.T) {
	const want = "lich-shell-70eb955f"
	if got := (Result{Path: "/usr/lib/lich/shell/lich-shell"}).profileKey(); got != want {
		t.Fatalf("profileKey = %q, want %q", got, want)
	}
}
