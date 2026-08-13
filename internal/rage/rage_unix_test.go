//go:build !windows

package rage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A log that is there but cannot be read is not the same as no log: the bundle
// drops that generation and carries everything else, because a report missing
// one log still beats no report at all.
// Windows has no equivalent: os.Chmod there only toggles the read-only
// attribute, which does not stop a read.
func TestAnUnreadableLogIsSkippedNotFatal(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root reads through a mode bit that denies everyone else")
	}
	configDir := lichDir(t, "live line\n", "older line\n")
	live := filepath.Join(configDir, "lich", "lich.log")
	if err := os.Chmod(live, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(live, 0o600) })

	files, report := bundle(t, collector(t, configDir, nil, false), "r")

	if _, ok := files["r/logs/lich.log"]; ok {
		t.Error("bundle carries a log generation it could not read")
	}
	if !strings.Contains(files["r/logs/lich.log.old"], "older line") {
		t.Error("the readable generation was dropped with the unreadable one")
	}
	if _, ok := files["r/report.md"]; !ok {
		t.Error("an unreadable log took the whole report with it")
	}
	if report.LogPath != live {
		t.Errorf("log path = %q, want %q — the file exists, it just would not open", report.LogPath, live)
	}
}
