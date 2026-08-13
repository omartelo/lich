//go:build !windows

package singleton

import (
	"os"
	"path/filepath"
	"testing"
)

// A runtime file that cannot be stat'ed is not a file that is not there. The
// notice a crash raises is the whole point of the check, so an unreadable
// directory must not be read as "the previous run exited cleanly".
// Windows has no equivalent: os.Chmod there only toggles the read-only
// attribute, which does not stop a stat.
func TestAnUnreadableRuntimeDirIsNotACleanExit(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root stats through a mode bit that denies everyone else")
	}
	configDir := writeConfig(t)
	if _, err := Write(configDir, 47821, "tok"); err != nil {
		t.Fatalf("Write: %v", err)
	}
	dir := filepath.Join(configDir, "lich")
	if err := os.Chmod(dir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	if !UncleanExit(configDir, "") {
		t.Fatal("UncleanExit over an unreadable runtime dir = false, want true")
	}
}
