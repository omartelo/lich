package logging

import (
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
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

// logDir is a temp dir for a test that lets Init succeed. Init points the
// runtime's crash output at the log file, and that keeps a duplicate of the
// file's handle open for as long as the registration stands — on Windows a
// file another handle still holds cannot be removed, so TempDir's cleanup
// fails unless the registration is dropped first. Registered after TempDir on
// purpose: cleanups run last-in-first-out, so this one runs before the removal.
func logDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Cleanup(func() { _ = debug.SetCrashOutput(nil, debug.CrashOptions{}) })
	return dir
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

// TestPathIsWhatInitWrites proves the path the frontend reveals is the file
// Init actually opened, under both the dev and the release name — the two
// would drift apart the moment they were spelled out twice.
func TestPathIsWhatInitWrites(t *testing.T) {
	for _, dev := range []string{"", "1"} {
		restoreDefault(t)
		t.Setenv("LICH_DEV", dev)
		dir := logDir(t)

		closer, err := Init(dir)
		if err != nil {
			t.Fatalf("LICH_DEV=%q Init: %v", dev, err)
		}
		defer closer.Close()

		if _, err := os.Stat(Path(dir)); err != nil {
			t.Errorf("LICH_DEV=%q: Path reports %q, which Init did not write: %v", dev, Path(dir), err)
		}
	}
}

// TestInitWritesRecordWithSource proves a record lands in the file carrying
// the message, the attributes and the source file:line the audit trail
// promises.
func TestInitWritesRecordWithSource(t *testing.T) {
	restoreDefault(t)
	t.Setenv("LICH_DEV", "")
	t.Setenv("LICH_LOG_LEVEL", "")
	dir := logDir(t)

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
	dir := logDir(t)
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

// TestInitReportsAFailedRotation proves a rotation that cannot happen is
// reported rather than silently skipped: Init still returns a working stderr
// logger, but the caller is told why the file half is missing. The .old name is
// held by a non-empty directory, which no rename can replace.
func TestInitReportsAFailedRotation(t *testing.T) {
	restoreDefault(t)
	t.Setenv("LICH_DEV", "")
	dir := logDir(t)
	path := filepath.Join(dir, "lich.log")
	if err := os.WriteFile(path, []byte("old generation\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(path, maxLogSize); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(path+".old", "occupied"), 0o700); err != nil {
		t.Fatal(err)
	}

	closer, err := Init(dir)

	if err == nil {
		t.Fatal("Init = nil error, want the rotation failure reported")
	}
	if !strings.Contains(err.Error(), "rotate log") {
		t.Errorf("error = %q, want it to name the rotation", err)
	}
	if closer != nil {
		t.Error("Init returned a closer for a file it never opened")
		_ = closer.Close()
	}
	// Logging must survive it: stderr alone is still a logger.
	slog.Info("a record after a failed rotation")
}

// TestInitKeepsUndersizedLog proves a log below the threshold is appended to
// rather than rotated. Rotating on every boot would overwrite .old with the
// session that just started — destroying the generation a bug report is told
// to attach, and the one RevealLog puts in view.
func TestInitKeepsUndersizedLog(t *testing.T) {
	restoreDefault(t)
	t.Setenv("LICH_DEV", "")
	dir := logDir(t)
	path := filepath.Join(dir, "lich.log")
	if err := os.WriteFile(path, []byte("previous generation\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	closer, err := Init(dir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer closer.Close()

	if _, err := os.Stat(path + ".old"); !os.IsNotExist(err) {
		t.Errorf("undersized log was rotated: stat(.old) = %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if !strings.HasPrefix(string(content), "previous generation") {
		t.Errorf("existing log not appended to:\n%s", content)
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

// crashProbeEnv hands the child half of TestInitRoutesPanicToLogFile the
// directory to log into; empty means this process is the parent.
const crashProbeEnv = "LICH_TEST_CRASH_DIR"

// TestInitRoutesPanicToLogFile proves an unhandled panic reaches the log file
// rather than only the stderr a windowsgui process does not have. Runs the
// crash in a subprocess, because a panic ends the process it happens in.
func TestInitRoutesPanicToLogFile(t *testing.T) {
	if dir := os.Getenv(crashProbeEnv); dir != "" {
		if _, err := Init(dir); err != nil {
			t.Fatalf("Init: %v", err)
		}
		panic("crash probe")
	}

	dir := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestInitRoutesPanicToLogFile$")
	cmd.Env = append(os.Environ(), crashProbeEnv+"="+dir, "LICH_DEV=")
	if err := cmd.Run(); err == nil {
		t.Fatal("child exited cleanly, want the panic to kill it")
	}

	content, err := os.ReadFile(filepath.Join(dir, "lich.log"))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	// The message and a stack: a trace without the panic value names no cause.
	for _, want := range []string{"crash probe", "goroutine "} {
		if !strings.Contains(string(content), want) {
			t.Errorf("panic did not reach the log file, missing %q:\n%s", want, content)
		}
	}
}
