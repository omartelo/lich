package logging

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// restoreDefault snapshots slog's process-global default logger, which Init
// replaces, so tests leave the world as they found it.
func restoreDefault(t *testing.T) {
	t.Helper()
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
}

// TestParseLevel proves the LICH_LOG_LEVEL values map onto slog levels and
// that anything unrecognized falls back to Info.
func TestParseLevel(t *testing.T) {
	cases := []struct {
		in   string
		want slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"WARN", slog.LevelWarn},
		{"Error", slog.LevelError},
		{"info", slog.LevelInfo},
		{"", slog.LevelInfo},
		{"verbose", slog.LevelInfo},
	}
	for _, tc := range cases {
		if got := parseLevel(tc.in); got != tc.want {
			t.Errorf("parseLevel(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// TestFileName proves the dev split mirrors the store's database naming.
func TestFileName(t *testing.T) {
	if got := fileName(false); got != "lich.log" {
		t.Errorf("fileName(false) = %q", got)
	}
	if got := fileName(true); got != "lich-dev.log" {
		t.Errorf("fileName(true) = %q", got)
	}
}

// TestInitWritesRecordWithSource proves a record lands in the file carrying
// the message, the attributes and the source file:line the audit trail
// promises.
func TestInitWritesRecordWithSource(t *testing.T) {
	restoreDefault(t)
	t.Setenv("LICH_DEV", "")
	t.Setenv("LICH_LOG_LEVEL", "")
	dir := t.TempDir()

	closer, err := Init(dir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer closer.Close()

	slog.Info("probe message", "key", "value")

	content, err := os.ReadFile(filepath.Join(dir, "lich.log"))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	for _, want := range []string{"probe message", "key=value", "logging_test.go"} {
		if !strings.Contains(string(content), want) {
			t.Errorf("log file missing %q:\n%s", want, content)
		}
	}
}

// TestInitRotatesOversizedLog proves an outgrown log is renamed to .old and a
// fresh file starts, keeping exactly one previous generation.
func TestInitRotatesOversizedLog(t *testing.T) {
	restoreDefault(t)
	t.Setenv("LICH_DEV", "")
	dir := t.TempDir()
	path := filepath.Join(dir, "lich.log")
	if err := os.WriteFile(path, []byte("old generation\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Truncate grows the file sparsely — no need to write 5MB.
	if err := os.Truncate(path, maxLogSize); err != nil {
		t.Fatal(err)
	}

	closer, err := Init(dir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer closer.Close()

	old, err := os.ReadFile(path + ".old")
	if err != nil {
		t.Fatalf("previous generation not kept: %v", err)
	}
	if !strings.HasPrefix(string(old), "old generation") {
		t.Errorf(".old does not carry the previous content")
	}
	if info, err := os.Stat(path); err != nil || info.Size() >= maxLogSize {
		t.Errorf("fresh log not started: size=%v err=%v", info, err)
	}
}

// failingWriter always errors, standing in for the dead stderr of a
// windowsgui process.
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, os.ErrInvalid
}

// TestBestEffortSwallowsMirrorFailure proves a dead mirror destination never
// fails the write — the exact contract the file half of the log depends on
// when the Windows GUI build has no console.
func TestBestEffortSwallowsMirrorFailure(t *testing.T) {
	n, err := bestEffort{failingWriter{}}.Write([]byte("record"))
	if err != nil || n != len("record") {
		t.Fatalf("Write = (%d, %v), want (%d, nil)", n, err, len("record"))
	}
}

// TestInitSurvivesUnwritableDir proves logging still works on stderr when the
// file half cannot exist: an error comes back, the closer is nil, and the
// default logger is usable.
func TestInitSurvivesUnwritableDir(t *testing.T) {
	restoreDefault(t)
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocked")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	closer, err := Init(filepath.Join(blocker, "sub"))
	if err == nil {
		t.Fatal("Init under a file path: want error")
	}
	if closer != nil {
		t.Fatal("closer should be nil when the file half failed")
	}
	slog.Info("must not panic")
}
