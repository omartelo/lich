//go:build !windows

package doctor

import (
	"os"
	"testing"
)

// Windows has no equivalent: os.Chmod there only toggles the read-only
// attribute, which Windows ignores when creating files inside a directory, so
// the probe in checkHome would find the directory writable and be right.
func TestAnUnwritableHomeStopsALaunch(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root writes through a read-only mode bit")
	}
	configDir := t.TempDir()
	if err := os.Chmod(configDir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(configDir, 0o700) })

	d := healthy(t)
	d.configDir = configDir

	checks := statuses(d.Run())

	if checks["home"].Status != Fail {
		t.Errorf("home = %s (%s), want fail", checks["home"].Status, checks["home"].Detail)
	}
	// The log lives under the same directory, so it cannot be opened either —
	// and that is a warning, because lich still runs on stderr.
	if checks["log"].Status != Warn {
		t.Errorf("log = %s (%s), want warn", checks["log"].Status, checks["log"].Detail)
	}
}
