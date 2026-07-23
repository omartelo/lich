// The suite spawns /bin/cat and /bin/sh under real PTYs, so it is Unix-only;
// the pure helpers it also covers (resumeArgs, childEnv, resolveCommand) are
// platform-independent logic exercised here all the same. A Windows CI run
// needs its own conpty-backed spawn tests before this tag can narrow.
//go:build !windows

package terminal

import (
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/omartelo/lich/internal/events"
)

// stubBins is a Store returning a fixed binary path, for tests that never
// spawn. Its persistence methods are no-ops — none of these tests exercise the
// SessionStart or ai-title paths.
type stubBins struct{ bin string }

func (s stubBins) ProviderBin(_, _ string) string            { return s.bin }
func (s stubBins) SetProviderSession(_, _ string) error      { return nil }
func (s stubBins) SetSessionTitle(_, _ string) (bool, error) { return false, nil }

// TestChildEnvStripsAppImageVars proves the AppImage runtime variables that break
// mise/asdf shims are dropped while the real user environment is passed through.
func TestChildEnvStripsAppImageVars(t *testing.T) {
	in := []string{
		"PATH=/usr/bin",
		"ARGV0=lich.AppImage",
		"APPIMAGE=/tmp/lich.AppImage",
		"APPDIR=/tmp/.mount_lich",
		"OWD=/home/user",
		"HOME=/home/user",
	}
	got := strings.Join(childEnv(in), "\n")
	for _, want := range []string{"PATH=/usr/bin", "HOME=/home/user"} {
		if !strings.Contains(got, want) {
			t.Errorf("childEnv dropped %q", want)
		}
	}
	for _, gone := range []string{"ARGV0", "APPIMAGE", "APPDIR", "OWD"} {
		if strings.Contains(got, gone+"=") {
			t.Errorf("childEnv leaked AppImage var %q", gone)
		}
	}
}

// TestChildEnvOutsideAppImageIsUntouched proves the deb/rpm/dev case: without
// APPDIR nothing is dropped or rewritten, even values that look like mount
// paths or AppImage-ish keys the user happens to have set.
func TestChildEnvOutsideAppImageIsUntouched(t *testing.T) {
	in := []string{
		"HOME=/home/user",
		"LD_LIBRARY_PATH=/opt/lib:/tmp/.mount_other/usr/lib",
		"WEBKIT_DISABLE_DMABUF_RENDERER=1",
	}
	got := childEnv(in)
	if strings.Join(got, "\n") != strings.Join(in, "\n") {
		t.Errorf("childEnv without APPDIR rewrote env:\n%v\nwant\n%v", got, in)
	}
}

// TestChildEnvScrubsMountPaths proves path lists lose only the entries under
// the AppImage mount: user-set entries survive, values reduced to nothing are
// dropped, and unrelated values are never rewritten.
func TestChildEnvScrubsMountPaths(t *testing.T) {
	const mount = "/tmp/.mount_lich"
	in := []string{
		"APPDIR=" + mount,
		// User-set, not runtime-injected: must survive even inside an AppImage.
		"WEBKIT_DISABLE_DMABUF_RENDERER=1",
		"TARGET_APPIMAGE=/home/user/Applications/lich.AppImage",
		"REDIRECT_APPIMAGE=/home/user/Applications/lich.AppImage",
		"DESKTOPINTEGRATION=AppImageLauncher",
		// AppRun's "${LD_LIBRARY_PATH:-}" on an unset var leaves a trailing
		// empty entry; everything points into the mount, so the var must go.
		"LD_LIBRARY_PATH=" + mount + "/usr/lib/x86_64-linux-gnu:" + mount + "/usr/lib:",
		"PATH=" + mount + "/usr/bin:/usr/local/bin:/usr/bin",
		"XDG_DATA_DIRS=" + mount + "/usr/share:/usr/local/share:/usr/share",
		"GDK_PIXBUF_MODULE_FILE=" + mount + "/usr/lib/gdk-pixbuf-2.0/2.10.0/loaders.cache",
		"GREP_COLORS=ms=01;31:mc=01;31",
		"HOME=/home/user",
	}
	got := strings.Join(childEnv(in), "\n")

	for _, want := range []string{
		"PATH=/usr/local/bin:/usr/bin",
		"XDG_DATA_DIRS=/usr/local/share:/usr/share",
		"GREP_COLORS=ms=01;31:mc=01;31",
		"HOME=/home/user",
		"WEBKIT_DISABLE_DMABUF_RENDERER=1",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("childEnv output missing %q:\n%s", want, got)
		}
	}
	for _, gone := range []string{
		"LD_LIBRARY_PATH", "GDK_PIXBUF_MODULE_FILE",
		"TARGET_APPIMAGE", "REDIRECT_APPIMAGE",
		"DESKTOPINTEGRATION", "APPDIR",
	} {
		if strings.Contains(got, gone+"=") {
			t.Errorf("childEnv leaked %q:\n%s", gone, got)
		}
	}
	if strings.Contains(got, mount) {
		t.Errorf("childEnv leaked a mount path:\n%s", got)
	}
}

// TestChildEnvKeepsUserLibraryPath proves a user's own LD_LIBRARY_PATH suffix
// survives the scrub — AppRun prepends the mount to whatever was already set.
func TestChildEnvKeepsUserLibraryPath(t *testing.T) {
	in := []string{
		"APPDIR=/tmp/.mount_lich",
		"LD_LIBRARY_PATH=/tmp/.mount_lich/usr/lib:/opt/cuda/lib64",
	}
	got := strings.Join(childEnv(in), "\n")
	if !strings.Contains(got, "LD_LIBRARY_PATH=/opt/cuda/lib64") {
		t.Errorf("childEnv lost the user's LD_LIBRARY_PATH:\n%s", got)
	}
}

// TestNewSessionEnv proves the service derives its session environment at
// construction: cleaned of AppImage leakage and terminated by TERM.
func TestNewSessionEnv(t *testing.T) {
	svc := New(stubBins{}, []string{"APPDIR=/tmp/.mount_lich", "ARGV0=lich.AppImage", "HOME=/home/user"}, events.New())
	got := strings.Join(svc.env, "\n")
	if strings.Contains(got, "ARGV0=") || strings.Contains(got, "APPDIR=") {
		t.Errorf("session env leaked AppImage vars:\n%s", got)
	}
	if !strings.Contains(got, "HOME=/home/user") || !strings.Contains(got, "TERM=xterm-256color") {
		t.Errorf("session env missing HOME or TERM:\n%s", got)
	}
}

// TestOperationsOnUnknownSessionAreNoops proves Write/Resize/Close on a session
// that was never started return nil instead of panicking on a missing PTY.
func TestOperationsOnUnknownSessionAreNoops(t *testing.T) {
	svc := New(stubBins{}, nil, events.New())
	if err := svc.Write("ghost", "hi"); err != nil {
		t.Errorf("Write unknown = %v, want nil", err)
	}
	if err := svc.Resize("ghost", 80, 24); err != nil {
		t.Errorf("Resize unknown = %v, want nil", err)
	}
	if err := svc.Close("ghost"); err != nil {
		t.Errorf("Close unknown = %v, want nil", err)
	}
	if err := svc.SetVisible("ghost", true); err != nil {
		t.Errorf("SetVisible unknown = %v, want nil", err)
	}
}

// TestSetVisibleReachesCoalescer proves the service routes visibility flips to
// the session's coalescer: output buffered while hidden is flushed when the
// session is made visible.
func TestSetVisibleReachesCoalescer(t *testing.T) {
	emit, emits := captureEmit(1)
	out := newCoalescer(emit, time.Hour, time.Hour)
	out.SetVisible(false)
	out.Write([]byte("pending"))

	svc := New(stubBins{}, nil, events.New())
	sess := spawnSession(t)
	sess.out = out
	svc.sessions["s1"] = sess

	if err := svc.SetVisible("s1", true); err != nil {
		t.Fatalf("SetVisible = %v, want nil", err)
	}
	select {
	case got := <-emits:
		if string(got) != "pending" {
			t.Errorf("flushed %q, want %q", got, "pending")
		}
	default:
		t.Error("SetVisible(true) did not flush the coalescer")
	}
}

// spawnSession starts /bin/cat under a PTY and returns a live session without
// going through Start, so no stream() goroutine emits events.
func spawnSession(t *testing.T) *session {
	t.Helper()
	p, err := startPTY(ptySpec{bin: "/bin/cat", cols: 80, rows: 24})
	if err != nil {
		t.Fatalf("startPTY: %v", err)
	}
	t.Cleanup(func() { _ = p.Close() })
	return &session{pty: p, done: make(chan struct{})}
}

// TestWriteResizeCloseOnLiveSession drives a real session end to end: input is
// written, the window is resized and Close reaps the shell and drops it.
func TestWriteResizeCloseOnLiveSession(t *testing.T) {
	svc := New(stubBins{}, nil, events.New())
	svc.sessions["s1"] = spawnSession(t)

	if err := svc.Write("s1", "hello"); err != nil {
		t.Errorf("Write = %v, want nil", err)
	}
	if err := svc.Resize("s1", 100, 40); err != nil {
		t.Errorf("Resize = %v, want nil", err)
	}
	if err := svc.Close("s1"); err != nil {
		t.Errorf("Close = %v, want nil", err)
	}
	if svc.ptyOf("s1") != nil {
		t.Error("session still present after Close")
	}
}

// TestStartIsNoopWhenAlreadyRunning proves Start returns without spawning a
// second shell for a session ID that is already tracked.
func TestStartIsNoopWhenAlreadyRunning(t *testing.T) {
	svc := New(stubBins{}, nil, events.New())
	sess := spawnSession(t)
	svc.sessions["s1"] = sess

	if err := svc.Start("s1", "p1", "", "", "", 80, 24); err != nil {
		t.Errorf("Start(running) = %v, want nil", err)
	}
	if svc.sessions["s1"] != sess {
		t.Error("Start replaced the running session")
	}
}

// stayAliveBin writes a script that outlives Start, so the spawned session is
// still in the map when the test inspects it. A binary that exits on an
// unknown flag would race stream()'s cleanup.
func stayAliveBin(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fake-claude")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nsleep 30\n"), 0o755); err != nil {
		t.Fatalf("write stub bin: %v", err)
	}
	return path
}

// spawnedArgs returns the argv of a session's spawned process. It reaches
// through the seam to the Unix implementation — these tests only run where a
// real PTY exists, and argv is not part of the ptyHandle contract.
func spawnedArgs(t *testing.T, svc *Service, id string) []string {
	t.Helper()
	p, ok := svc.sessions[id].pty.(*unixPTY)
	if !ok {
		t.Fatalf("session %q pty is %T, want *unixPTY", id, svc.sessions[id].pty)
	}
	return p.cmd.Args
}

// TestStartPassesResumeToTheProcess proves the resume id reaches the spawned
// binary's argv — the wiring resumeArgs' unit test cannot see.
func TestStartPassesResumeToTheProcess(t *testing.T) {
	bin := stayAliveBin(t)
	svc := New(stubBins{bin: bin}, nil, events.New())
	t.Cleanup(func() { _ = svc.Close("s1") })

	if err := svc.Start("s1", "p1", t.TempDir(), "claude", "abc-123", 80, 24); err != nil {
		t.Fatalf("Start = %v, want nil", err)
	}

	svc.mu.Lock()
	got := spawnedArgs(t, svc, "s1")
	svc.mu.Unlock()

	want := []string{bin, resumeFlag, "abc-123"}
	if !slices.Equal(got, want) {
		t.Errorf("spawned argv = %v, want %v", got, want)
	}
}

// TestStartWithoutResumeSpawnsBare proves a session with no id to resume spawns
// the binary alone, with no dangling flag.
func TestStartWithoutResumeSpawnsBare(t *testing.T) {
	bin := stayAliveBin(t)
	svc := New(stubBins{bin: bin}, nil, events.New())
	t.Cleanup(func() { _ = svc.Close("s1") })

	if err := svc.Start("s1", "p1", t.TempDir(), "claude", "", 80, 24); err != nil {
		t.Fatalf("Start = %v, want nil", err)
	}

	svc.mu.Lock()
	got := spawnedArgs(t, svc, "s1")
	svc.mu.Unlock()

	if !slices.Equal(got, []string{bin}) {
		t.Errorf("spawned argv = %v, want %v", got, []string{bin})
	}
}

// TestResolveBin proves an empty custom path falls back to the provider's
// default binary (and to defaultBin for an unknown kind), while a configured
// path is passed through unchanged.
func TestResolveBin(t *testing.T) {
	if got := resolveBin("claude", ""); got != defaultBin {
		t.Errorf("resolveBin(claude, %q) = %q, want %q", "", got, defaultBin)
	}
	if got := resolveBin("codex", ""); got != "codex" {
		t.Errorf("resolveBin(codex, %q) = %q, want codex", "", got)
	}
	if got := resolveBin("mystery", ""); got != defaultBin {
		t.Errorf("resolveBin(unknown, %q) = %q, want %q", "", got, defaultBin)
	}
	if got := resolveBin("claude", "/opt/claude.sh"); got != "/opt/claude.sh" {
		t.Errorf("resolveBin custom = %q, want %q", got, "/opt/claude.sh")
	}
}

// TestResolveCommand proves kind selects between the user's shell and the
// Claude Code binary, with fallbacks when either source is empty.
func TestResolveCommand(t *testing.T) {
	cases := []struct {
		name, kind, bin, shell, want string
	}{
		{"claude default", "claude", "", "/bin/zsh", defaultBin},
		{"claude custom bin", "claude", "/opt/claude.sh", "/bin/zsh", "/opt/claude.sh"},
		{"codex default", "codex", "", "/bin/zsh", "codex"},
		{"crush custom bin", "crush", "/opt/crush", "/bin/zsh", "/opt/crush"},
		{"unknown kind falls back", "mystery", "", "/bin/zsh", defaultBin},
		{"shell from env", KindShell, "/opt/claude.sh", "/bin/zsh", "/bin/zsh"},
		{"shell fallback", KindShell, "", "", defaultShell},
	}
	for _, tc := range cases {
		if got := resolveCommand(tc.kind, tc.bin, tc.shell); got != tc.want {
			t.Errorf("%s: resolveCommand(%q, %q, %q) = %q, want %q",
				tc.name, tc.kind, tc.bin, tc.shell, got, tc.want)
		}
	}
}

// TestResumeArgs proves --resume is Claude-only: a claude session resumes when
// an id is given, and neither a shell nor any other provider ever grows the flag
// (it is Claude Code's, and a stray id must not reach codex/opencode/crush).
func TestResumeArgs(t *testing.T) {
	cases := []struct {
		name, kind, resume string
		want               []string
	}{
		{"claude fresh", "claude", "", nil},
		{"claude resume", "claude", "abc-123", []string{resumeFlag, "abc-123"}},
		{"codex never resumes", "codex", "abc-123", nil},
		{"shell never resumes", KindShell, "abc-123", nil},
		{"shell fresh", KindShell, "", nil},
	}
	for _, tc := range cases {
		got := resumeArgs(tc.kind, tc.resume)
		if !slices.Equal(got, tc.want) {
			t.Errorf("%s: resumeArgs(%q, %q) = %v, want %v",
				tc.name, tc.kind, tc.resume, got, tc.want)
		}
	}
}

// TestPTYEcho proves the core assumption of the service: a process spawns
// under a PTY and its output is readable. If this platform's startPTY breaks,
// this fails.
func TestPTYEcho(t *testing.T) {
	const marker = "lich-pty-test"

	p, err := startPTY(ptySpec{
		bin:  "/bin/sh",
		args: []string{"-c", "echo " + marker},
		cols: 80,
		rows: 24,
	})
	if err != nil {
		t.Fatalf("startPTY: %v", err)
	}
	t.Cleanup(func() { _ = p.Close() })

	done := make(chan string, 1)
	go func() {
		out, _ := io.ReadAll(p)
		done <- string(out)
	}()

	select {
	case out := <-done:
		if !strings.Contains(out, marker) {
			t.Errorf("PTY output %q does not contain marker %q", out, marker)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out reading PTY output")
	}
}

// TestSessionEnvInjectsCoordinates proves a spawned PTY gets the loopback
// coordinates a Claude Code hook needs, without aliasing the shared base env.
func TestSessionEnvInjectsCoordinates(t *testing.T) {
	s := &Service{env: []string{"A=1"}, ws: &transport{port: 4321, token: "tok"}}
	env := s.sessionEnv("sess")

	want := map[string]bool{
		"A=1":                  true,
		"LICH_PORT=4321":       true,
		"LICH_TOKEN=tok":       true,
		"LICH_SESSION_ID=sess": true,
	}
	for _, e := range env {
		delete(want, e)
	}
	if len(want) != 0 {
		t.Fatalf("missing env entries %v (got %v)", want, env)
	}
	if len(s.env) != 1 || s.env[0] != "A=1" {
		t.Fatalf("shared base env was mutated: %v", s.env)
	}
}

// TestSessionEnvNoTransport proves that without a transport there is nothing to
// report to, so the base env is returned unchanged (the hook will no-op).
func TestSessionEnvNoTransport(t *testing.T) {
	s := &Service{env: []string{"A=1"}}
	env := s.sessionEnv("sess")
	if len(env) != 1 || env[0] != "A=1" {
		t.Fatalf("expected base env unchanged, got %v", env)
	}
}
