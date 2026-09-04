// The suite spawns /bin/cat and /bin/sh under real PTYs, so it is Unix-only;
// the pure helpers it also covers (resumeArgs, childEnv, resolveCommand) are
// platform-independent logic exercised here all the same. A Windows CI run
// needs its own conpty-backed spawn tests before this tag can narrow.
//go:build !windows

package terminal

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

// stubBins is a Store returning a fixed binary path and project directory,
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
	bin             string
	projectPath     string
	providerSession string
	providerErr     error
	model           string
	entrypoint      string
	sandbox         string
	sandboxOn       bool
	sshAgent        bool
	ghToken         bool
	ghAccount       string
	skipPerms       bool
	costOn          bool
	ledgers         map[string]stubLedger
	ports           map[string]int
	// Whole seconds per session, the shape store.AddHandsOn accumulates into.
	// Nil until a test cares: the flush only ever writes for a session the
	// accumulator actually counted something for.
	handsOn map[string]int64
	// One field per cost method, because the three failures are three different
	// stories: a ledger that cannot be read, one that cannot be written, and a
	// total that cannot be summed. A single error field would let a test claim
	// the path it never reached.
	ledgerErr, saveLedgerErr, sessionCostErr error
}

// projectWithSetup is a project checkout shipping .lich/setup-worktree.sh —
// the source the spawn reads since the script moved out of the store and into
// the repository.
func projectWithSetup(t *testing.T, script string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".lich"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".lich", "setup-worktree.sh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
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
func (s stubBins) SessionCustomBin(_ string) bool       { return s.bin != "" }
func (s stubBins) SkipPermissions(_, _, _ string) bool  { return s.skipPerms }
func (s stubBins) ProjectPath(_ string) string          { return s.projectPath }
func (s stubBins) SessionModel(_ string) string         { return s.model }
func (s stubBins) SessionEntrypoint(_ string) string    { return s.entrypoint }
func (s stubBins) SessionSandbox(_ string) string       { return s.sandbox }
func (s stubBins) SetSessionSandbox(_, _ string) error  { return nil }
func (s stubBins) SandboxDefault(_, _, _ string) bool   { return s.sandboxOn }
func (s stubBins) SandboxSSHAgent(_ string) bool        { return s.sshAgent }
func (s stubBins) SandboxGHToken(_ string) bool         { return s.ghToken }
func (s stubBins) GHAccountForPath(_ string) string     { return s.ghAccount }
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
	if s.ledgerErr != nil {
		return 0, "", 0, s.ledgerErr
	}
	ledger := s.ledgers[sessionID+"\x00"+transcriptID]
	return ledger.offset, ledger.lastMessage, ledger.cost, nil
}

func (s stubBins) SaveCostLedger(
	sessionID, transcriptID string, offset int64, lastMessage string, cost float64,
) error {
	if s.saveLedgerErr != nil {
		return s.saveLedgerErr
	}
	s.ledgers[sessionID+"\x00"+transcriptID] = stubLedger{offset, lastMessage, cost}
	return nil
}

func (s stubBins) SessionCost(sessionID string) (float64, error) {
	if s.sessionCostErr != nil {
		return 0, s.sessionCostErr
	}
	total := 0.0
	for key, ledger := range s.ledgers {
		if strings.HasPrefix(key, sessionID+"\x00") {
			total += ledger.cost
		}
	}
	return total, nil
}

func (s stubBins) AddHandsOn(sessionID string, seconds int64) error {
	if s.handsOn != nil {
		s.handsOn[sessionID] += seconds
	}
	return nil
}

func (s stubBins) HandsOn(sessionID string) (int64, error) { return s.handsOn[sessionID], nil }

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
	spawnPins(t, got, want...)
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
	spawnPins(t, got, want...)
}

// TestStartCarriesTheBriefingIntoTheSpawn proves lich's briefing reaches the
// process itself, and not only providerArgs. Every argv assertion around it
// goes through spawnPins, which drops the briefing before comparing — so
// nothing else here would notice it being lost between the two.
func TestStartCarriesTheBriefingIntoTheSpawn(t *testing.T) {
	bin := stayAliveBin(t)
	svc := New(stubBins{bin: bin}, nil, events.New())
	t.Cleanup(func() { _ = svc.Close("s1") })

	if err := svc.Start("s1", "p1", t.TempDir(), "claude", "", "", false, 80, 24); err != nil {
		t.Fatalf("Start = %v, want nil", err)
	}

	svc.mu.Lock()
	got := spawnedArgs(t, svc, "s1")
	svc.mu.Unlock()

	flag := slices.Index(got, "--append-system-prompt")
	if flag < 0 || flag+1 >= len(got) {
		t.Fatalf("spawned argv = %v, want a briefing", got)
	}
	if !strings.Contains(got[flag+1], "subagent") {
		t.Errorf("briefing = %q, want the line it exists to draw", got[flag+1])
	}
}

// TestStartWithSetupWrapsTheSpawn proves the setup flag reroutes the spawn
// through sh -c with the project's script ahead of the exec'd provider — the
// wiring wrapSetup's unit test cannot see.
func TestStartWithSetupWrapsTheSpawn(t *testing.T) {
	bin := stayAliveBin(t)
	svc := New(stubBins{bin: bin, projectPath: projectWithSetup(t, "echo setup-ran")}, nil, events.New())
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
	spawnPins(t, got, want...)
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

// TestProviderKindOutranksTheReport proves a session-start report cannot repaint
// a card lich itself chose the provider for. A harness can run another harness's
// hooks — Cursor CLI executes every Claude Code hook on the machine, the user's
// own and each installed plugin's — so the lich plugin's own script reports
// `claude` from inside a Cursor session. What lich spawned wins; only a shell,
// where lich genuinely does not know what is running inside, wears the report.
func TestProviderKindOutranksTheReport(t *testing.T) {
	svc := &Service{}
	for id, kind := range map[string]string{
		"cursor-card": providers.Cursor,
		"shell-card":  KindShell,
		"odd-card":    "something-else",
	} {
		svc.spawns.Store(id, spawn{kind: kind})
	}
	cases := []struct {
		name, id, reported, want string
	}{
		{"a provider card keeps its own kind", "cursor-card", providers.Claude, providers.Cursor},
		{"and does so even when the report agrees", "cursor-card", providers.Cursor, providers.Cursor},
		{"a shell wears what reported", "shell-card", providers.Claude, providers.Claude},
		{"an unregistered kind is no answer", "odd-card", providers.Claude, providers.Claude},
		{"a report that raced the PTY's exit", "gone-card", providers.Claude, providers.Claude},
	}
	for _, tc := range cases {
		if got := svc.providerKind(tc.id, tc.reported); got != tc.want {
			t.Errorf("%s: providerKind(%q, %q) = %q, want %q",
				tc.name, tc.id, tc.reported, got, tc.want)
		}
	}
}

// The invariant the note on Service.turns states: a hook must never queue behind
// a PTY spawn. spawnSession holds mu across startPTY, so asking mu what a
// session runs would hold the report — and with it the agent's next step — for
// however long another session takes to spawn.
func TestAReportNeverQueuesBehindASpawn(t *testing.T) {
	svc := &Service{}
	svc.spawns.Store("s1", spawn{kind: providers.Cursor})

	// Stands in for a spawn in flight: the same lock, held by another session.
	svc.mu.Lock()
	defer svc.mu.Unlock()

	answered := make(chan string, 1)
	go func() { answered <- svc.kindOf("s1") }()
	select {
	case got := <-answered:
		if got != providers.Cursor {
			t.Errorf("kindOf = %q, want %q", got, providers.Cursor)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a state report queued behind a PTY spawn")
	}
}

// A kind that outlived its PTY would have providerKind answering for a session
// that is gone, which is the one case the report is the better answer.
func TestClosingASessionDropsItsKind(t *testing.T) {
	svc := &Service{sessions: map[string]*session{}}
	svc.spawns.Store("s1", spawn{kind: providers.Cursor})
	if err := svc.Close("s1"); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if got := svc.kindOf("s1"); got != "" {
		t.Errorf("a closed session still runs %q", got)
	}
}

// TestResumeArgs proves each provider resumes in its own spelling — a flag of
// its own for Claude Code, Antigravity, oh-my-pi and Cursor CLI, a subcommand
// for Codex, one they happen to share for opencode and Crush — and that a kind
// with none wired never grows one: a shell must not be handed a stray id.
func TestResumeArgs(t *testing.T) {
	cases := []struct {
		name, kind, resume string
		want               []string
	}{
		{"claude fresh", "claude", "", nil},
		{"claude resume", "claude", "abc-123", []string{"--resume", "abc-123"}},
		{"codex fresh", "codex", "", nil},
		{"codex resume", "codex", "abc-123", []string{"resume", "abc-123"}},
		{"antigravity fresh", "antigravity", "", nil},
		{"antigravity resume", "antigravity", "abc-123", []string{"--conversation", "abc-123"}},
		{"omp fresh", "omp", "", nil},
		{"omp resume", "omp", "abc-123", []string{"-r", "abc-123"}},
		{"opencode fresh", "opencode", "", nil},
		{"opencode resume", "opencode", "ses_0031a382dffe", []string{"--session", "ses_0031a382dffe"}},
		{"crush fresh", "crush", "", nil},
		{"crush resume", "crush", "abc-123", []string{"--session", "abc-123"}},
		{"cursor fresh", "cursor", "", nil},
		// Cursor spells it exactly as Claude Code does, and is pinned here for
		// the same reason Antigravity's skip-permissions flag is: a shared
		// literal is what makes a lookup returning the wrong provider's flag
		// invisible everywhere else. Its own value is optional — `--resume`
		// alone opens a picker — so an empty id must never reach argv.
		{"cursor resume", "cursor", "abc-123", []string{"--resume", "abc-123"}},
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
// roster — that a name which would be parsed as a flag never reaches argv, and
// that a resuming session is not named at all: Claude Code restores the name
// from the transcript, and naming it again would overwrite a /rename the user
// typed inside the session.
func TestNameArgs(t *testing.T) {
	cases := []struct {
		name, kind, peer, resume string
		want                     []string
	}{
		{"claude named", "claude", "lich-4f2a", "", []string{"--name", "lich-4f2a"}},
		{"claude unnamed", "claude", "", "", nil},
		{"claude blank name", "claude", "   ", "", nil},
		{"claude name trimmed", "claude", " lich-4f2a ", "", []string{"--name", "lich-4f2a"}},
		{"claude flag-like name", "claude", "--dangerously-skip-permissions", "", nil},
		{"claude resuming keeps its own name", "claude", "lich-4f2a", "conv-1", nil},
		{"codex has no roster", "codex", "lich-4f2a", "", nil},
		{"opencode has no roster", "opencode", "lich-4f2a", "", nil},
		{"shell has no roster", KindShell, "lich-4f2a", "", nil},
	}
	for _, tc := range cases {
		got := nameArgs(tc.kind, tc.peer, tc.resume)
		if !slices.Equal(got, tc.want) {
			t.Errorf("%s: nameArgs(%q, %q, %q) = %v, want %v",
				tc.name, tc.kind, tc.peer, tc.resume, got, tc.want)
		}
	}
}

// TestSkipPermissionArgs proves each provider gets its own spelling and only
// when the setting is on. The literals are pinned rather than read back from
// skipPermissionFlags: this is the flag that hands an agent the machine, and a
// test that follows the map would keep passing while the map handed opencode
// Claude Code's flag. A kind with no spelling — a shell — gets none.
func TestSkipPermissionArgs(t *testing.T) {
	cases := []struct {
		name, kind string
		skip       bool
		want       []string
	}{
		{"claude off", "claude", false, nil},
		{"claude on", "claude", true, []string{"--dangerously-skip-permissions"}},
		{"codex off", "codex", false, nil},
		{"codex on", "codex", true, []string{"--dangerously-bypass-approvals-and-sandbox"}},
		{"antigravity off", "antigravity", false, nil},
		// Antigravity spells it exactly as Claude Code does. Pinned twice on
		// purpose: the shared literal is what makes a lookup returning the wrong
		// provider's flag invisible everywhere else.
		{"antigravity on", "antigravity", true, []string{"--dangerously-skip-permissions"}},
		{"opencode on", "opencode", true, []string{"--auto"}},
		{"crush on", "crush", true, []string{"--yolo"}},
		{"oh-my-pi off", "omp", false, nil},
		{"oh-my-pi on", "omp", true, []string{"--auto-approve"}},
		// --yolo is Cursor's own alias for --force; lich passes the canonical
		// spelling, which is not the one Crush answers to above.
		{"cursor off", "cursor", false, nil},
		{"cursor on", "cursor", true, []string{"--force"}},
		// Kiro's is the only one that is not a "dangerous"/"yolo"/"force" word,
		// and it does not fully deliver: with it on, Kiro's TUI still opens on a
		// confirmation the user has to accept once (docs/ceilings.md).
		{"kiro off", "kiro", false, nil},
		{"kiro on", "kiro", true, []string{"--trust-all-tools"}},
		{"shell is never wired", KindShell, true, nil},
	}
	for _, tc := range cases {
		got := skipPermissionArgs(tc.kind, tc.skip)
		if !slices.Equal(got, tc.want) {
			t.Errorf("%s: skipPermissionArgs(%q, %v) = %v, want %v",
				tc.name, tc.kind, tc.skip, got, tc.want)
		}
	}
}

// TestStartPassesSkipPermissionsToTheProcess proves the stored setting reaches
// the spawned binary's argv — the wiring skipPermissionArgs' unit test cannot
// see, and the one that decides whether a user who ticked the box actually gets
// an agent that stops asking.
func TestStartPassesSkipPermissionsToTheProcess(t *testing.T) {
	bin := stayAliveBin(t)
	svc := New(stubBins{bin: bin, skipPerms: true}, nil, events.New())
	t.Cleanup(func() { _ = svc.Close("s1") })

	if err := svc.Start("s1", "p1", t.TempDir(), "claude", "", "", false, 80, 24); err != nil {
		t.Fatalf("Start = %v, want nil", err)
	}

	svc.mu.Lock()
	got := spawnedArgs(t, svc, "s1")
	svc.mu.Unlock()

	// As above: the MCP registration follows, pinned by its own test. The flag
	// has to land ahead of it — --mcp-config reads everything after it as another
	// config path.
	want := []string{bin, "--dangerously-skip-permissions", "--mcp-config"}
	spawnPins(t, got, want...)
}

// TestStartPassesNameToTheProcess proves the peer name reaches the spawned
// binary's argv when a session is born, in the order claude parses.
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
	spawnPins(t, got, want...)
}

// TestModelArgs pins each provider's own flag, and the two kinds that must never
// be handed one: Crush spells --model only on its non-interactive `run`
// subcommand, and a shell is not a provider. The flag literals are pinned rather
// than read back from modelFlags, for the same reason skipPermissionArgs' test
// pins its own — a test that follows the map cannot see the map hand a provider
// somebody else's flag.
//
// The model values are only carriers for the passthrough rule: an alias, a full
// name and a provider/model pair all reach the argv untouched. lich knows no
// model names, so none of these is a catalogue entry to keep up to date.
func TestModelArgs(t *testing.T) {
	cases := []struct {
		name, kind, model string
		want              []string
	}{
		{"claude, an alias", providers.Claude, "opus", []string{"--model", "opus"}},
		{"codex, a full name", providers.Codex, "gpt-5.2", []string{"--model", "gpt-5.2"}},
		{"opencode, a provider/model pair", providers.OpenCode, "openai/gpt-5.2",
			[]string{"--model", "openai/gpt-5.2"}},
		{"oh-my-pi, a fuzzy match", providers.OMP, "opus", []string{"--model", "opus"}},
		{"antigravity, a full name", providers.Antigravity, "gemini-3.7-flash-high",
			[]string{"--model", "gemini-3.7-flash-high"}},
		{"cursor, a parameterized name", providers.Cursor, "claude-opus-4-8[effort=high]",
			[]string{"--model", "claude-opus-4-8[effort=high]"}},
		{"crush takes none at spawn", providers.Crush, "opus", nil},
		{"a shell is not a provider", KindShell, "opus", nil},
		{"no model named", providers.Claude, "", nil},
		{"blank model", providers.Claude, "   ", nil},
		{"model trimmed", providers.Claude, " opus ", []string{"--model", "opus"}},
		{"flag-like model", providers.Claude, "--dangerously-skip-permissions", nil},
	}
	for _, tc := range cases {
		got := modelArgs(tc.kind, tc.model)
		if !slices.Equal(got, tc.want) {
			t.Errorf("%s: modelArgs(%q, %q) = %v, want %v", tc.name, tc.kind, tc.model, got, tc.want)
		}
	}
}

// TestSupportsModel pins what the spawn service rejects on: a caller that names
// a model for a provider lich cannot pass one to hears about it instead of
// getting a session quietly running the provider's default.
func TestSupportsModel(t *testing.T) {
	for _, kind := range []string{
		providers.Claude, providers.Codex, providers.Antigravity, providers.OpenCode, providers.OMP,
	} {
		if !SupportsModel(kind) {
			t.Errorf("SupportsModel(%q) = false, want true", kind)
		}
	}
	for _, kind := range []string{providers.Crush, KindShell, "gemini"} {
		if SupportsModel(kind) {
			t.Errorf("SupportsModel(%q) = true, want false", kind)
		}
	}
}

// TestStartPassesTheStoredModelToTheProcess proves the model recorded on the row
// reaches the spawned binary's argv. It is read from the store rather than
// passed to Start precisely so it survives a respawn, and this is the wiring
// that carries it there.
func TestStartPassesTheStoredModelToTheProcess(t *testing.T) {
	bin := stayAliveBin(t)
	svc := New(stubBins{bin: bin, model: "opus"}, nil, events.New())
	t.Cleanup(func() { _ = svc.Close("s1") })

	if err := svc.Start("s1", "p1", t.TempDir(), "claude", "", "", false, 80, 24); err != nil {
		t.Fatalf("Start = %v, want nil", err)
	}

	svc.mu.Lock()
	got := spawnedArgs(t, svc, "s1")
	svc.mu.Unlock()

	// Ahead of the registration, as every other flag is: --mcp-config reads what
	// follows it as another config path, so a model behind it is a model lost.
	want := []string{bin, "--model", "opus", "--mcp-config"}
	spawnPins(t, got, want...)
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

// spawnPins asserts a spawn's argv: it starts with the binary, carries these
// flags in this order, and ends with the MCP registration plus its one config
// value.
//
// lich's own briefing is dropped before the comparison. Every Claude Code spawn
// carries it, and each of these tests owns one flag of its own — pinning the
// briefing's presence six more times would only mean six failures the day its
// wording moves. What it says is pinned once, by
// TestTheBriefingGoesToTheProvidersThatTakeOne.
func spawnPins(t *testing.T, got []string, want ...string) {
	t.Helper()
	trimmed := make([]string, 0, len(got))
	for i := 0; i < len(got); i++ {
		if got[i] == "--append-system-prompt" {
			i++ // and the text it carries
			continue
		}
		trimmed = append(trimmed, got[i])
	}
	if len(trimmed) != len(want)+1 || !slices.Equal(trimmed[:len(want)], want) {
		t.Errorf("spawned argv = %v, want %v followed by one config", got, want)
	}
}

// TestProviderArgsRegistersTheMCPServer proves each provider that can be handed
// an MCP server at spawn is handed lich's, spelled its own way.
func TestProviderArgsRegistersTheMCPServer(t *testing.T) {
	const bin = "/usr/bin/lich"

	claude := providerArgs(providers.Claude, "", "", "", bin, "", false)
	at := slices.Index(claude, "--mcp-config")
	if at < 0 || at+1 >= len(claude) {
		t.Fatalf("claude args = %v", claude)
	}
	var config struct {
		MCPServers map[string]struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal([]byte(claude[at+1]), &config); err != nil {
		t.Fatalf("--mcp-config value is not JSON: %v (%q)", err, claude[at+1])
	}
	server, ok := config.MCPServers["lich"]
	if !ok {
		t.Fatalf("no lich server in %q", claude[at+1])
	}
	if server.Command != bin || !slices.Equal(server.Args, []string{"mcp"}) {
		t.Errorf("server = %+v, want %q mcp", server, bin)
	}
	if strings.Contains(claude[at+1], "token") {
		t.Errorf("a secret reached the argv, which /proc exposes: %q", claude[at+1])
	}

	codex := providerArgs(providers.Codex, "", "", "", bin, "", false)
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
	claude := providerArgs(providers.Claude, "lich-4f2a", "conv-1", "", "/usr/bin/lich", "", true)
	if claude[len(claude)-2] != "--mcp-config" {
		t.Errorf("--mcp-config is not last, so it eats what follows: %v", claude)
	}
	for _, want := range []string{"--resume", "--dangerously-skip-permissions"} {
		if !slices.Contains(claude, want) {
			t.Errorf("claude args lost %q: %v", want, claude)
		}
	}

	// A resuming session is not named (nameArgs), so --name is pinned on the
	// spawn that carries it: a session being born.
	born := providerArgs(providers.Claude, "lich-4f2a", "", "", "/usr/bin/lich", "", true)
	if born[len(born)-2] != "--mcp-config" {
		t.Errorf("--mcp-config is not last, so it eats what follows: %v", born)
	}
	if !slices.Contains(born, "--name") {
		t.Errorf("claude args lost --name: %v", born)
	}

	codex := providerArgs(providers.Codex, "", "conv-1", "", "/usr/bin/lich", "", false)
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

	// The model is the one flag Codex takes on both sides of the subcommand
	// (`codex resume --help` lists it), and it goes after: a flag the resumed
	// conversation's own parser accepts is one less thing riding on where the
	// global options end.
	resumed := providerArgs(providers.Codex, "", "conv-1", "gpt-5.2", "/usr/bin/lich", "", false)
	model := slices.Index(resumed, "--model")
	if model < 0 || model < slices.Index(resumed, "resume") {
		t.Errorf("codex args = %v, want --model after the resume subcommand", resumed)
	}
}

// TestProviderArgsWithoutARegistration proves the providers with no way to be
// told at spawn are spawned exactly as before, and that a lich which cannot
// name its own binary registers nothing rather than something broken.
//
// bare is the stronger claim: nothing at all on the command line. It does not
// hold for the two that take a briefing without taking an MCP server — for
// those, what is proven is that no registration rode along with it.
func TestProviderArgsWithoutARegistration(t *testing.T) {
	tests := []struct {
		name string
		kind string
		bin  string
		bare bool
	}{
		{"opencode has no flag for it", providers.OpenCode, "/usr/bin/lich", true},
		{"crush has no flag for it", providers.Crush, "/usr/bin/lich", true},
		{"a shell session is not a provider", KindShell, "/usr/bin/lich", true},
		{"oh-my-pi takes a briefing but no server", providers.OMP, "/usr/bin/lich", false},
		{"no binary to point at", providers.Claude, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := providerArgs(tt.kind, "", "", "", tt.bin, "", false)
			if tt.bare && len(args) != 0 {
				t.Errorf("args = %v, want none", args)
			}
			for _, flag := range []string{claudeMCPFlag, codexConfigFlag} {
				if slices.Contains(args, flag) {
					t.Errorf("args = %v, want no MCP registration", args)
				}
			}
		})
	}
}

// TestTheBriefingGoesToTheProvidersThatTakeOne proves the two halves of the
// briefing: who is handed it at all, and that its wording follows what this
// spawn registered — a provider given no MCP server is pointed at the command
// line, because naming a tool it does not have is the one thing that leaves it
// with no route at all.
func TestTheBriefingGoesToTheProvidersThatTakeOne(t *testing.T) {
	briefed := map[string]string{
		providers.Claude: "tools in your list",
		providers.OMP:    "lich open --worktree",
	}
	for kind, want := range briefed {
		t.Run(kind, func(t *testing.T) {
			args := providerArgs(kind, "", "", "", "/usr/bin/lich", "", false)
			flag := slices.Index(args, "--append-system-prompt")
			if flag < 0 || flag+1 >= len(args) {
				t.Fatalf("args = %v, want a briefing", args)
			}
			if !strings.Contains(args[flag+1], want) {
				t.Errorf("briefing = %q, want it to name %q", args[flag+1], want)
			}
		})
	}

	for _, kind := range []string{providers.Codex, providers.OpenCode, providers.Crush, KindShell} {
		t.Run(kind+" takes none", func(t *testing.T) {
			args := providerArgs(kind, "", "", "", "/usr/bin/lich", "", false)
			if slices.Contains(args, "--append-system-prompt") {
				t.Errorf("args = %v, want no briefing: %s has no flag that appends one", args, kind)
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

// TestSessionStateReachesItsWatcher proves the seam the relay hangs off: a hook
// report must reach SetSessionState, not only the window's event channel.
// Everything downstream is unit-tested against a fake, so a break here would
// look exactly like a feature that simply never fires — which is what an
// unanswered request looked like on the first real run.
func TestSessionStateReachesItsWatcher(t *testing.T) {
	svc := New(stubBins{}, nil, events.New())
	if svc.ws == nil {
		t.Skip("no transport on this machine")
	}
	type report struct{ id, state string }
	seen := make(chan report, 4)
	svc.SetSessionState(func(id, state string) { seen <- report{id, state} })

	url := fmt.Sprintf("http://127.0.0.1:%d/hook?token=%s", svc.ws.port, svc.ws.token)
	for _, state := range []string{"busy", "done"} {
		body := fmt.Sprintf(`{"session_id":"sess","state":%q}`, state)
		resp, err := http.Post(url, "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatalf("post %s: %v", state, err)
		}
		_ = resp.Body.Close()
	}

	for _, want := range []report{{"sess", "busy"}, {"sess", "done"}} {
		select {
		case got := <-seen:
			if got != want {
				t.Fatalf("watcher saw %+v, want %+v", got, want)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("watcher never saw %+v", want)
		}
	}
}

// TestSessionStateWithoutAWatcherIsHarmless proves the default: nothing is wired
// until main wires it, and a report arriving first must not take the hook down.
func TestSessionStateWithoutAWatcherIsHarmless(t *testing.T) {
	svc := New(stubBins{}, nil, events.New())
	if svc.ws == nil {
		t.Skip("no transport on this machine")
	}

	url := fmt.Sprintf("http://127.0.0.1:%d/hook?token=%s", svc.ws.port, svc.ws.token)
	resp, err := http.Post(url, "application/json", strings.NewReader(`{"session_id":"s","state":"busy"}`))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}
}

// The size a session starts at is a contract with the window, not an
// implementation detail: a session drawn into a grid the window does not have
// is repainted on its first view, and whatever its provider had already written
// is gone from the screen (see sizeFor).

func TestSizeForKeepsAMeasuredSize(t *testing.T) {
	svc := New(stubBins{}, nil, events.New())

	svc.mu.Lock()
	cols, rows := svc.sizeFor(174, 51)
	svc.mu.Unlock()

	if cols != 174 || rows != 51 {
		t.Errorf("sizeFor(174, 51) = %dx%d, want the caller's own terminal", cols, rows)
	}
}

func TestSizeForFallsBackWhenNothingHasBeenMeasured(t *testing.T) {
	svc := New(stubBins{}, nil, events.New())

	svc.mu.Lock()
	cols, rows := svc.sizeFor(0, 0)
	svc.mu.Unlock()

	// Pinned, not read off the constants: a conventional terminal is the whole
	// reason this number is safe to hand a TUI that nobody is watching yet.
	if cols != 80 || rows != 24 {
		t.Errorf("sizeFor(0, 0) with no window = %dx%d, want 80x24", cols, rows)
	}
}

func TestSizeForCopiesTheWindowsLastSize(t *testing.T) {
	svc := New(stubBins{}, nil, events.New())

	svc.mu.Lock()
	svc.sizeFor(174, 51)
	cols, rows := svc.sizeFor(0, 0)
	svc.mu.Unlock()

	if cols != 174 || rows != 51 {
		t.Errorf("sizeFor(0, 0) = %dx%d, want the window's own 174x51", cols, rows)
	}
}

// A resize is the window measuring its terminal, which is the freshest size
// lich has — including for the session an agent opens next, whose own PTY
// nobody is looking at.
func TestResizeRecordsTheWindowsSize(t *testing.T) {
	svc := New(stubBins{}, nil, events.New())

	if err := svc.Resize("ghost", 174, 51); err != nil {
		t.Fatalf("Resize = %v, want nil", err)
	}

	svc.mu.Lock()
	cols, rows := svc.sizeFor(0, 0)
	svc.mu.Unlock()
	if cols != 174 || rows != 51 {
		t.Errorf("a session spawned after the resize starts at %dx%d, want 174x51", cols, rows)
	}
}

// TestStartWithoutASizeCopiesTheWindow drives the whole path: a card the window
// measured, then a session opened with no terminal of its own.
func TestStartWithoutASizeCopiesTheWindow(t *testing.T) {
	bin := stayAliveBin(t)
	svc := New(stubBins{bin: bin}, nil, events.New())
	t.Cleanup(func() { _ = svc.Close("s1"); _ = svc.Close("s2") })

	if err := svc.Start("s1", "p1", t.TempDir(), "claude", "", "", false, 174, 51); err != nil {
		t.Fatalf("Start = %v, want nil", err)
	}
	if err := svc.Start("s2", "p1", t.TempDir(), "claude", "", "", false, 0, 0); err != nil {
		t.Fatalf("Start without a size = %v, want nil", err)
	}

	svc.mu.Lock()
	cols, rows := svc.lastCols, svc.lastRows
	svc.mu.Unlock()
	if cols != 174 || rows != 51 {
		t.Errorf("the agent's session moved the window's size to %dx%d", cols, rows)
	}
}

// TestReadyWaitsForTheSetupScript drives the whole seam: a session spawned with
// the project's setup script is live but has nothing on the other end that can
// read a message, until the wrapper's marker comes through its own output.
func TestReadyWaitsForTheSetupScript(t *testing.T) {
	bin := stayAliveBin(t)
	svc := New(stubBins{bin: bin, projectPath: projectWithSetup(t, "echo installing")}, nil, events.New())
	t.Cleanup(func() { _ = svc.Close("s1") })

	if err := svc.Start("s1", "p1", t.TempDir(), "claude", "", "", true, 80, 24); err != nil {
		t.Fatalf("Start = %v, want nil", err)
	}
	if !svc.Live("s1") {
		t.Fatal("the session is not live")
	}
	if svc.Ready("s1") {
		t.Fatal("a session still running its setup script was offered work")
	}

	svc.mu.Lock()
	sess := svc.sessions["s1"]
	svc.mu.Unlock()
	svc.noteOutput("s1", sess, []byte("Progress: resolved 551"+setupDone))

	// The marker is the exec, not the prompt: the provider draws its opening
	// screen from here, and a message written into that is the one that landed
	// on screen as literal paste markers.
	if svc.Ready("s1") {
		t.Error("work was offered the instant the setup script exec'd the provider")
	}
	// This session has a real PTY behind it, still writing the script's own
	// output and marker on its own schedule, so the settle runs from whenever
	// that last byte lands rather than from here.
	waitFor(t, func() bool { return svc.Ready("s1") }, "the provider to settle")
}

// TestReadyWaitsForTheProviderToStopDrawing covers the spawn with no setup
// script at all: the provider still takes the terminal over, and a message
// written before it does is lost the same way.
func TestReadyWaitsForTheProviderToStopDrawing(t *testing.T) {
	svc := New(stubBins{}, nil, events.New())
	sess := &session{}

	svc.noteOutput("s1", sess, []byte("Claude Code v2.1.227"))
	svc.mu.Lock()
	svc.sessions["s1"] = sess
	svc.mu.Unlock()

	if svc.Ready("s1") {
		t.Error("a session still drawing its opening screen was offered work")
	}
	time.Sleep(readySettle + 100*time.Millisecond)
	if !svc.Ready("s1") {
		t.Error("a session that went quiet was not offered work")
	}
}

// Once a session has been ready it stays ready: a working agent draws
// continuously, and a target mid-turn has always been written to — its provider
// queues the input and answers a turn later.
func TestReadyStaysTrueThroughABusyTurn(t *testing.T) {
	svc := New(stubBins{}, nil, events.New())
	sess := &session{lastOut: time.Now().Add(-time.Second)}
	svc.mu.Lock()
	svc.sessions["s1"] = sess
	svc.mu.Unlock()

	if !svc.Ready("s1") {
		t.Fatal("a settled session was not ready")
	}
	svc.noteOutput("s1", sess, []byte("...thinking..."))
	if !svc.Ready("s1") {
		t.Error("a session that started working again stopped accepting work")
	}
}

// The setup script's marker is the exec, and the exec is not instant: the image
// is replaced, a runtime starts, a splash is composed, and none of that writes a
// byte. Measured from the script's last line, that silence is long enough to
// pass for a provider that has finished drawing — so the first byte of the
// splash would clear the settle, and the message the relay then types lands in a
// TUI that has not taken the terminal yet.
func TestTheWaitForTheProviderToStartIsNotItsQuiet(t *testing.T) {
	svc := New(stubBins{}, nil, events.New())
	sess := &session{settingUp: true}
	svc.mu.Lock()
	svc.sessions["s1"] = sess
	svc.mu.Unlock()

	svc.noteOutput("s1", sess, []byte("Progress: resolved 551"+setupDone))
	time.Sleep(readySettle + 100*time.Millisecond)
	svc.noteOutput("s1", sess, []byte("Claude Code v2.1.227"))
	if svc.Ready("s1") {
		t.Error("the wait for the provider to start was credited as the provider going quiet")
	}

	// Its own quiet, once it has drawn itself, still counts.
	time.Sleep(readySettle + 100*time.Millisecond)
	if !svc.Ready("s1") {
		t.Error("the session stayed unready after its provider settled")
	}
}

// The quiet a session settles into is recorded as it passes, not sampled when
// somebody finally asks. A session live for hours is at its prompt the whole
// time, but the first thing ever asked of it is a relayed message — and by then
// it is mid-turn, redrawing a spinner every few frames. Sampled there it looks
// like a provider that never took the terminal, and the message it was sent is
// refused instead of queued.
func TestReadyRemembersAQuietThatAlreadyPassed(t *testing.T) {
	svc := New(stubBins{}, nil, events.New())
	sess := &session{}
	svc.mu.Lock()
	svc.sessions["s1"] = sess
	svc.mu.Unlock()

	svc.noteOutput("s1", sess, []byte("Claude Code v2.1.227"))
	time.Sleep(readySettle + 100*time.Millisecond)
	// The turn starts. From here the PTY never goes quiet again, and this is the
	// first time anyone asks whether the session can be given work.
	svc.noteOutput("s1", sess, []byte("...thinking..."))
	svc.noteOutput("s1", sess, []byte("...thinking..."))

	if !svc.Ready("s1") {
		t.Error("a session that sat at its prompt for a whole turn was refused work")
	}
}

func TestReadyWaitsOnlyTheSettleWithoutASetupScript(t *testing.T) {
	bin := stayAliveBin(t)
	svc := New(stubBins{bin: bin}, nil, events.New())
	t.Cleanup(func() { _ = svc.Close("s1") })

	if err := svc.Start("s1", "p1", t.TempDir(), "claude", "", "", false, 80, 24); err != nil {
		t.Fatalf("Start = %v, want nil", err)
	}
	// No setup script to wait out, but the provider still has to take the
	// terminal: readiness is the quiet after it draws, never the spawn itself.
	time.Sleep(readySettle + 100*time.Millisecond)
	if !svc.Ready("s1") {
		t.Error("a session with no setup script to wait for was held back")
	}
}

func TestReadyIsFalseForASessionThatIsNotRunning(t *testing.T) {
	svc := New(stubBins{}, nil, events.New())

	if svc.Ready("ghost") {
		t.Error("a session with no PTY was reported ready for work")
	}
}

// An unconfined session already runs with the user's whole environment. Asking
// gh for a token there would spend a subprocess to hand over something the
// session never lacked.
func TestSandboxCredentialsAreOnlyForConfinedSessions(t *testing.T) {
	svc := New(stubBins{sshAgent: true, ghToken: true, ghAccount: "github.com/someone"}, nil, events.New())
	if got := svc.sandboxCredentials("p1", "/repo", false); got != (sandboxCreds{}) {
		t.Errorf("an unconfined session was handed %+v, want nothing", got)
	}
	if got := svc.sandboxCredentials("p1", "/repo", true); !got.sshAgent {
		t.Error("a confined session was refused the agent its project turned on")
	}
}

// TestClosableState pins which reports survive per provider. Cursor CLI is the
// only one filtered, because it is the only one whose reports lich does not
// register: it runs Claude Code's installed plugin, which registers all of them,
// while the CLI itself delivers only SessionStart, PreToolUse, PostToolUse and
// SessionEnd. A `busy` from a tool call would therefore never be followed by the
// `done` that ends it, and the spinner would stay on the card for the rest of
// the session.
func TestClosableState(t *testing.T) {
	cases := []struct {
		name, kind, state string
		want              bool
	}{
		{"a claude turn begins", providers.Claude, statusBusy, true},
		{"and ends", providers.Claude, statusDone, true},
		{"crush reports what it can", providers.Crush, statusBusy, true},
		// The two Cursor delivers but cannot end.
		{"a cursor tool call does not begin a turn", providers.Cursor, statusBusy, false},
		{"nor does anything else", providers.Cursor, statusDone, false},
		{"cursor raises no approval event either", providers.Cursor, statusWaiting, false},
		// SessionEnd is the one Cursor both reports and means.
		{"a cursor session ending is reported", providers.Cursor, statusIdle, true},
		{"a shell is not filtered", KindShell, statusBusy, true},
		{"nor is a session already gone", "", statusBusy, true},
	}
	for _, tc := range cases {
		if got := closableState(tc.kind, tc.state); got != tc.want {
			t.Errorf("%s: closableState(%q, %q) = %v, want %v",
				tc.name, tc.kind, tc.state, got, tc.want)
		}
	}
}
