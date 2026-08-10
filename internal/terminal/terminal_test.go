// The suite spawns /bin/cat and /bin/sh under real PTYs, so it is Unix-only;
// the pure helpers it also covers (resumeArgs, childEnv, resolveCommand) are
// platform-independent logic exercised here all the same. A Windows CI run
// needs its own conpty-backed spawn tests before this tag can narrow.
//go:build !windows

package terminal

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/omartelo/lich/internal/events"
	"github.com/omartelo/lich/internal/providers"
	"github.com/omartelo/lich/internal/shquote"
)

// stubBins is a Store returning a fixed binary path and worktree setup script,
// for tests that never spawn. Its write methods are no-ops — none of these tests
// exercise the SessionStart or ai-title paths — while providerSession and its
// error drive the context-usage read (usage_test.go).
//
// The cost side is a real (if tiny) implementation rather than a no-op: the
// ledger is what makes a second read resume instead of recount, so a stub that
// forgot it would let that contract pass untested. ledgers is a map so the
// value receivers can still write to it; a test that exercises cost builds one
// with newCostStore.
type stubBins struct {
	bin, setup      string
	projectPath     string
	providerSession string
	providerErr     error
	costOn          bool
	ledgers         map[string]stubLedger
	ports           map[string]int
}

// stubLedger mirrors one session_costs row.
type stubLedger struct {
	offset      int64
	lastMessage string
	cost        float64
}

// newCostStore is a stubBins with the cost readout on and an empty ledger.
func newCostStore(providerSession string) stubBins {
	return stubBins{
		providerSession: providerSession,
		costOn:          true,
		ledgers:         make(map[string]stubLedger),
	}
}

func (s stubBins) ProviderBin(_, _ string) string       { return s.bin }
func (s stubBins) ProjectPath(_ string) string          { return s.projectPath }
func (s stubBins) WorktreeSetup(_ string) string        { return s.setup }
func (s stubBins) SetProviderSession(_, _ string) error { return nil }

func (s stubBins) WorktreePorts() map[string]int {
	if s.ports == nil {
		return map[string]int{}
	}
	return s.ports
}

func (s stubBins) SetWorktreePort(path string, port int) error {
	if s.ports != nil {
		s.ports[path] = port
	}
	return nil
}

func (s stubBins) ProviderSession(_ string) (string, error) {
	return s.providerSession, s.providerErr
}
func (s stubBins) SetSessionTitle(_, _ string) (bool, error) { return false, nil }

func (s stubBins) CostReadout() bool { return s.costOn }

func (s stubBins) CostLedger(sessionID, transcriptID string) (int64, string, float64, error) {
	ledger := s.ledgers[sessionID+"\x00"+transcriptID]
	return ledger.offset, ledger.lastMessage, ledger.cost, nil
}

func (s stubBins) SaveCostLedger(
	sessionID, transcriptID string, offset int64, lastMessage string, cost float64,
) error {
	s.ledgers[sessionID+"\x00"+transcriptID] = stubLedger{offset, lastMessage, cost}
	return nil
}

func (s stubBins) SessionCost(sessionID string) (float64, error) {
	total := 0.0
	for key, ledger := range s.ledgers {
		if strings.HasPrefix(key, sessionID+"\x00") {
			total += ledger.cost
		}
	}
	return total, nil
}

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

	if err := svc.Start("s1", "p1", "", "", "", "", false, 80, 24); err != nil {
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

// echoBin writes a script that echoes its input like cat and ignores whatever
// arguments it is spawned with. A session spawn carries flags of lich's own now
// (the MCP registration), and plain /bin/cat would read those as file names,
// fail, and exit before the test could inspect the session.
func echoBin(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fake-claude")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexec cat\n"), 0o755); err != nil {
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

	if err := svc.Start("s1", "p1", t.TempDir(), "claude", "abc-123", "", false, 80, 24); err != nil {
		t.Fatalf("Start = %v, want nil", err)
	}

	svc.mu.Lock()
	got := spawnedArgs(t, svc, "s1")
	svc.mu.Unlock()

	// A spawn also registers lich's MCP server, whose content
	// TestProviderArgsRegistersTheMCPServer pins; this test owns the resume id
	// reaching argv ahead of it.
	want := []string{bin, "--resume", "abc-123", "--mcp-config"}
	if len(got) != len(want)+1 || !slices.Equal(got[:len(want)], want) {
		t.Errorf("spawned argv = %v, want %v followed by one config", got, want)
	}
}

// TestStartWithoutResumeSpawnsBare proves a session with no id to resume spawns
// the binary alone, with no dangling flag.
func TestStartWithoutResumeSpawnsBare(t *testing.T) {
	bin := stayAliveBin(t)
	svc := New(stubBins{bin: bin}, nil, events.New())
	t.Cleanup(func() { _ = svc.Close("s1") })

	if err := svc.Start("s1", "p1", t.TempDir(), "claude", "", "", false, 80, 24); err != nil {
		t.Fatalf("Start = %v, want nil", err)
	}

	svc.mu.Lock()
	got := spawnedArgs(t, svc, "s1")
	svc.mu.Unlock()

	// A claude spawn is never bare now: it carries lich's MCP registration,
	// whose content TestProviderArgsRegistersTheMCPServer pins. What this test
	// owns is that no resume flag was invented alongside it.
	want := []string{bin, "--mcp-config"}
	if len(got) != len(want)+1 || !slices.Equal(got[:len(want)], want) {
		t.Errorf("spawned argv = %v, want %v followed by one config", got, want)
	}
}

// TestStartWithSetupWrapsTheSpawn proves the setup flag reroutes the spawn
// through sh -c with the project's script ahead of the exec'd provider — the
// wiring wrapSetup's unit test cannot see.
func TestStartWithSetupWrapsTheSpawn(t *testing.T) {
	bin := stayAliveBin(t)
	svc := New(stubBins{bin: bin, setup: "echo setup-ran"}, nil, events.New())
	t.Cleanup(func() { _ = svc.Close("s1") })

	if err := svc.Start("s1", "p1", t.TempDir(), "claude", "", "", true, 80, 24); err != nil {
		t.Fatalf("Start = %v, want nil", err)
	}

	svc.mu.Lock()
	got := spawnedArgs(t, svc, "s1")
	svc.mu.Unlock()

	if len(got) != 3 || got[0] != "sh" || got[1] != "-c" {
		t.Fatalf("spawned argv = %v, want sh -c <cmd>", got)
	}
	for _, want := range []string{"echo setup-ran", "exec " + shquote.Quote(bin)} {
		if !strings.Contains(got[2], want) {
			t.Errorf("spawned command missing %q:\n%s", want, got[2])
		}
	}
}

// TestStartWithSetupButNoScriptSpawnsBare proves the flag is inert for a
// project with no setup script configured — no sh indirection sneaks in.
func TestStartWithSetupButNoScriptSpawnsBare(t *testing.T) {
	bin := stayAliveBin(t)
	svc := New(stubBins{bin: bin}, nil, events.New())
	t.Cleanup(func() { _ = svc.Close("s1") })

	if err := svc.Start("s1", "p1", t.TempDir(), "claude", "", "", true, 80, 24); err != nil {
		t.Fatalf("Start = %v, want nil", err)
	}

	svc.mu.Lock()
	got := spawnedArgs(t, svc, "s1")
	svc.mu.Unlock()

	// As above: the registration is expected, an sh indirection is not — argv
	// still starts at the provider binary itself.
	want := []string{bin, "--mcp-config"}
	if len(got) != len(want)+1 || !slices.Equal(got[:len(want)], want) {
		t.Errorf("spawned argv = %v, want %v followed by one config", got, want)
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

// TestResumeArgs proves each provider resumes in its own spelling — a flag for
// Claude Code, a subcommand for Codex — and that a provider with none wired
// never grows one: a shell, opencode or Crush must not be handed a stray id.
func TestResumeArgs(t *testing.T) {
	cases := []struct {
		name, kind, resume string
		want               []string
	}{
		{"claude fresh", "claude", "", nil},
		{"claude resume", "claude", "abc-123", []string{"--resume", "abc-123"}},
		{"codex fresh", "codex", "", nil},
		{"codex resume", "codex", "abc-123", []string{"resume", "abc-123"}},
		{"opencode never resumes", "opencode", "abc-123", nil},
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

// TestNameArgs proves only Claude Code is named — the one provider with a peer
// roster — and that a name which would be parsed as a flag never reaches argv.
func TestNameArgs(t *testing.T) {
	cases := []struct {
		name, kind, peer string
		want             []string
	}{
		{"claude named", "claude", "lich-4f2a", []string{"--name", "lich-4f2a"}},
		{"claude unnamed", "claude", "", nil},
		{"claude blank name", "claude", "   ", nil},
		{"claude name trimmed", "claude", " lich-4f2a ", []string{"--name", "lich-4f2a"}},
		{"claude flag-like name", "claude", "--dangerously-skip-permissions", nil},
		{"codex has no roster", "codex", "lich-4f2a", nil},
		{"opencode has no roster", "opencode", "lich-4f2a", nil},
		{"shell has no roster", KindShell, "lich-4f2a", nil},
	}
	for _, tc := range cases {
		got := nameArgs(tc.kind, tc.peer)
		if !slices.Equal(got, tc.want) {
			t.Errorf("%s: nameArgs(%q, %q) = %v, want %v",
				tc.name, tc.kind, tc.peer, got, tc.want)
		}
	}
}

// TestStartPassesNameToTheProcess proves the peer name reaches the spawned
// binary's argv beside the resume id, in the order claude parses.
func TestStartPassesNameToTheProcess(t *testing.T) {
	bin := stayAliveBin(t)
	svc := New(stubBins{bin: bin}, nil, events.New())
	t.Cleanup(func() { _ = svc.Close("s1") })

	if err := svc.Start("s1", "p1", t.TempDir(), "claude", "", "lich-4f2a", false, 80, 24); err != nil {
		t.Fatalf("Start = %v, want nil", err)
	}

	svc.mu.Lock()
	got := spawnedArgs(t, svc, "s1")
	svc.mu.Unlock()

	// As above: the registration follows, pinned by its own test.
	want := []string{bin, "--name", "lich-4f2a", "--mcp-config"}
	if len(got) != len(want)+1 || !slices.Equal(got[:len(want)], want) {
		t.Errorf("spawned argv = %v, want %v followed by one config", got, want)
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

// TestReserveWorktreePortKeepsReservation proves a checkout that already holds
// a port keeps it untouched — probing would read its own running dev server as
// "taken" and move the checkout off the port it is serving on.
func TestReserveWorktreePortKeepsReservation(t *testing.T) {
	store := stubBins{ports: map[string]int{"/src/repo": 24007}}
	s := &Service{store: store}
	if got := s.reserveWorktreePort("/src/repo/"); got != 24007 {
		t.Fatalf("reserveWorktreePort = %d, want the reserved 24007", got)
	}
	if store.ports["/src/repo"] != 24007 {
		t.Fatalf("reservation was rewritten to %d", store.ports["/src/repo"])
	}
}

// TestReserveWorktreePortRecordsAllocation proves a first-time checkout is
// written into the table under its cleaned path — without that write the next
// checkout to collide with it would never learn the port is spoken for.
func TestReserveWorktreePortRecordsAllocation(t *testing.T) {
	store := stubBins{ports: map[string]int{}}
	s := &Service{store: store}
	got := s.reserveWorktreePort("/src/repo/")
	if got < worktreePortBase || got >= worktreePortBase+worktreePortCount {
		t.Fatalf("reserveWorktreePort = %d, outside the window", got)
	}
	if store.ports["/src/repo"] != got {
		t.Fatalf("reserved %d but the table holds %v", got, store.ports)
	}
}

// TestReserveWorktreePortSeparatesCollidingCheckouts proves the end-to-end
// point of the reservation: two checkouts that hash to one port do not both
// leave with it.
func TestReserveWorktreePortSeparatesCollidingCheckouts(t *testing.T) {
	first := "/src/repo/feature-a"
	// A path is not free to choose — it has to hash onto first's port — so the
	// collision is manufactured by seeding the table with first's number under
	// a path that is not the one being allocated.
	store := stubBins{ports: map[string]int{"/src/other": worktreePort(first)}}
	s := &Service{store: store}
	if got := s.reserveWorktreePort(first); got == worktreePort(first) {
		t.Fatalf("reserveWorktreePort(%q) = %d, the port /src/other holds", first, got)
	}
}

// TestSessionEnvInjectsCoordinates proves a spawned PTY gets the loopback
// coordinates a Claude Code hook needs, its project's directory and its
// checkout's dev-server port, without aliasing the shared base env. The port is
// the one the checkout holds in the reservation table, not a number recomputed
// here.
func TestSessionEnvInjectsCoordinates(t *testing.T) {
	store := stubBins{projectPath: "/src/repo", ports: map[string]int{"/src/repo/wt": 24007}}
	s := &Service{env: []string{"A=1"}, store: store, ws: &transport{port: 4321, token: "tok"}}
	env := s.sessionEnv("sess", "p1", "/src/repo/wt")

	want := map[string]bool{
		"A=1":                        true,
		"LICH_PORT=4321":             true,
		"LICH_TOKEN=tok":             true,
		"LICH_SESSION_ID=sess":       true,
		"LICH_PROJECT_DIR=/src/repo": true,
		"LICH_WORKTREE_PORT=24007":   true,
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

// TestProviderArgsRegistersTheMCPServer proves each provider that can be handed
// an MCP server at spawn is handed lich's, spelled its own way.
func TestProviderArgsRegistersTheMCPServer(t *testing.T) {
	const bin = "/usr/bin/lich"

	claude := providerArgs(providers.Claude, "", "", bin)
	if len(claude) != 2 || claude[0] != "--mcp-config" {
		t.Fatalf("claude args = %v", claude)
	}
	var config struct {
		MCPServers map[string]struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal([]byte(claude[1]), &config); err != nil {
		t.Fatalf("--mcp-config value is not JSON: %v (%q)", err, claude[1])
	}
	server, ok := config.MCPServers["lich"]
	if !ok {
		t.Fatalf("no lich server in %q", claude[1])
	}
	if server.Command != bin || !slices.Equal(server.Args, []string{"mcp"}) {
		t.Errorf("server = %+v, want %q mcp", server, bin)
	}
	if strings.Contains(claude[1], "token") {
		t.Errorf("a secret reached the argv, which /proc exposes: %q", claude[1])
	}

	codex := providerArgs(providers.Codex, "", "", bin)
	want := []string{
		"-c", `mcp_servers.lich.command="/usr/bin/lich"`,
		"-c", `mcp_servers.lich.args=["mcp"]`,
	}
	if !slices.Equal(codex, want) {
		t.Errorf("codex args = %v, want %v", codex, want)
	}
}

// TestProviderArgsOrdersEachProvidersConstraint pins the two orderings that are
// not interchangeable: Claude Code's --mcp-config is variadic and swallows what
// follows it, and Codex reads resume as a subcommand that every global option
// must precede.
func TestProviderArgsOrdersEachProvidersConstraint(t *testing.T) {
	claude := providerArgs(providers.Claude, "lich-4f2a", "conv-1", "/usr/bin/lich")
	if claude[len(claude)-2] != "--mcp-config" {
		t.Errorf("--mcp-config is not last, so it eats what follows: %v", claude)
	}
	for _, want := range []string{"--name", "--resume"} {
		if !slices.Contains(claude, want) {
			t.Errorf("claude args lost %q: %v", want, claude)
		}
	}

	codex := providerArgs(providers.Codex, "", "conv-1", "/usr/bin/lich")
	resume := slices.Index(codex, "resume")
	if resume < 0 {
		t.Fatalf("codex args lost the resume subcommand: %v", codex)
	}
	if last := slices.Index(codex, "-c"); last > resume {
		t.Errorf("a global option follows the subcommand: %v", codex)
	}
	if codex[len(codex)-1] != "conv-1" {
		t.Errorf("codex args = %v, want the conversation id last", codex)
	}
}

// TestProviderArgsWithoutARegistration proves the providers with no way to be
// told at spawn are spawned exactly as before, and that a lich which cannot
// name its own binary registers nothing rather than something broken.
func TestProviderArgsWithoutARegistration(t *testing.T) {
	tests := []struct {
		name string
		kind string
		bin  string
	}{
		{"opencode has no flag for it", providers.OpenCode, "/usr/bin/lich"},
		{"crush has no flag for it", providers.Crush, "/usr/bin/lich"},
		{"oh-my-pi has no flag for it", providers.OMP, "/usr/bin/lich"},
		{"a shell session is not a provider", KindShell, "/usr/bin/lich"},
		{"no binary to point at", providers.Claude, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if args := providerArgs(tt.kind, "", "", tt.bin); len(args) != 0 {
				t.Errorf("args = %v, want none", args)
			}
		})
	}
}

// TestSessionEnvExportsTheBinary proves a session is told where the lich it
// belongs to lives. Its agent calls that path back to reach another session
// (internal/cli), and on a machine running an installed lich beside a dev build
// the bare name would resolve to the wrong one.
func TestSessionEnvExportsTheBinary(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Skipf("no executable path on this platform: %v", err)
	}
	s := &Service{env: []string{"A=1"}, store: stubBins{}, ws: &transport{port: 4321, token: "tok"}}

	var got string
	for _, e := range s.sessionEnv("sess", "p1", "") {
		if path, ok := strings.CutPrefix(e, "LICH_BIN="); ok {
			got = path
		}
	}
	if got != exe {
		t.Fatalf("LICH_BIN = %q, want the running binary %q", got, exe)
	}
}

// TestSessionEnvNoTransport proves that without a transport there is nothing to
// report to, so the hook coordinates are left out (the hook will no-op) — but
// the project directory and the checkout's dev-server port, which owe the
// transport nothing, still land.
func TestSessionEnvNoTransport(t *testing.T) {
	store := stubBins{projectPath: "/src/repo", ports: map[string]int{"/src/repo/wt": 24007}}
	s := &Service{env: []string{"A=1"}, store: store}
	env := s.sessionEnv("sess", "p1", "/src/repo/wt")

	want := []string{"A=1", "LICH_PROJECT_DIR=/src/repo", "LICH_WORKTREE_PORT=24007"}
	if !slices.Equal(env, want) {
		t.Fatalf("env = %v, want %v", env, want)
	}
	for _, e := range env {
		if strings.HasPrefix(e, "LICH_PORT=") || strings.HasPrefix(e, "LICH_TOKEN=") {
			t.Fatalf("transport coordinates leaked without a transport: %v", env)
		}
	}
}

// TestSessionEnvWithoutCwd proves a session with no start directory names no
// checkout, so no port is invented for it.
func TestSessionEnvWithoutCwd(t *testing.T) {
	s := &Service{env: []string{"A=1"}, store: stubBins{}}
	env := s.sessionEnv("sess", "p1", "")
	if len(env) != 1 || env[0] != "A=1" {
		t.Fatalf("expected base env unchanged, got %v", env)
	}
}

// TestSessionEnvWithoutProject proves a session whose project the store cannot
// resolve is left without LICH_PROJECT_DIR rather than handed an empty one: a
// setup script reading it would `cd` to the filesystem root.
func TestSessionEnvWithoutProject(t *testing.T) {
	s := &Service{env: []string{"A=1"}, store: stubBins{}}
	for _, e := range s.sessionEnv("sess", "gone", "") {
		if strings.HasPrefix(e, "LICH_PROJECT_DIR=") {
			t.Fatalf("exported %q for a project that does not resolve", e)
		}
	}
}
