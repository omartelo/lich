//go:build !windows

package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A probe lich can write but not delete leaves a file behind in the user's
// config dir on every run, so the directory is writable — the verdict — with a
// warning naming what was left. The write succeeds because the file already
// exists and is the user's; only unlinking it needs the directory.
func TestAProbeThatCannotBeRemovedWarns(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root unlinks through a read-only mode bit")
	}
	d := healthy(t)
	dir := filepath.Join(d.configDir, "lich")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	probe := filepath.Join(dir, ".doctor-probe")
	if err := os.WriteFile(probe, []byte("left over"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	checks := d.Run()

	got := statuses(checks)["home"]
	if got.Status != Warn || !strings.Contains(got.Detail, probe) {
		t.Errorf("home = %s (%s), want warn naming the leftover probe", got.Status, got.Detail)
	}
	if Failed(checks) {
		t.Error("a writable directory is reported as launch-stopping")
	}
}

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
