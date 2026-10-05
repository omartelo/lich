package cli

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/omartelo/lich/internal/project"
	"github.com/omartelo/lich/internal/relay"
	"github.com/omartelo/lich/internal/rpc"
	"github.com/omartelo/lich/internal/spawn"
	"github.com/omartelo/lich/internal/store"
	"github.com/omartelo/lich/internal/terminal"
)

// The tests above answer the CLI with canned JSON, which proves what it prints
// but not that the app understands what it asks. These drive the real
// dispatcher over a real socket, so an argument that moves position or changes
// type fails here rather than in a terminal.

type wiredSessions struct{}

func (wiredSessions) LoadState() ([]store.Project, error) {
	return []store.Project{{ID: "p1", Name: "lich", Sessions: []store.Session{
		{ID: "s1", Label: "sender", Kind: "claude"},
		{ID: "s2", Label: "docs", Kind: "codex"},
	}}}, nil
}

// Nothing here schedules a prompt; the relay only ever calls this to clear one.
func (wiredSessions) SetSessionSchedule(string, int64, string) error { return nil }

// No checkout here has a branch git could name.
func (wiredSessions) SessionBranch(string) string { return "" }

type wiredTerminal struct {
	mu    sync.Mutex
	typed string
}

// Pointer receiver like Write's: a value receiver would copy the mutex beside
// it on every call, which is a race the moment the two are used together.
func (*wiredTerminal) Live(string) bool { return true }

func (*wiredTerminal) Ready(string) bool { return true }

func (*wiredTerminal) QuietFor(string) time.Duration { return time.Hour }

// Nothing renamed itself in these tests, so the roster stays on the name lich
// derives — which is the one the wiring under test addresses.
func (*wiredTerminal) AgentName(string) string { return "" }

// Nobody is typing at these sessions, so the hold has nothing to keep back.
func (*wiredTerminal) HoldInput(string) func() { return func() {} }

// No mod polls from these sessions, so every message is typed and recorded.
func (*wiredTerminal) SubmitPrompt(string, string, *relay.Notification) (relay.HandedPrompt, error) {
	return relay.HandedPrompt{}, relay.ErrNoMod
}

func (*wiredTerminal) ModAttached(string) bool { return false }

func (w *wiredTerminal) Write(_, data string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.typed += data
	return nil
}

func (w *wiredTerminal) message() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.typed
}

// wiredLich serves the relay behind the real RPC dispatcher and returns the
// session environment a PTY would carry, plus the terminal it types into.
func wiredLich(t *testing.T) (func(string) string, *wiredTerminal) {
	t.Helper()
	term := &wiredTerminal{}
	dispatcher := rpc.New()
	// No events sink: these tests are about the wire between the CLI and the
	// dispatcher, and the window is not on this side of it.
	dispatcher.Register("relay", relay.New(wiredSessions{}, term, nil))

	server := httptest.NewServer(dispatcher)
	t.Cleanup(server.Close)

	port := strconv.Itoa(server.Listener.Addr().(*net.TCPAddr).Port)
	return sessionEnv(port), term
}

// sessionEnv is the environment a PTY of the lich listening on port carries,
// for the session "s1".
func sessionEnv(port string) func(string) string {
	return func(key string) string {
		switch key {
		case "LICH_PORT":
			return port
		case "LICH_TOKEN":
			return "tok"
		case "LICH_SESSION_ID":
			return "s1"
		}
		return ""
	}
}

func TestSessionsOverTheRealDispatcher(t *testing.T) {
	env, _ := wiredLich(t)

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"sessions"}, "test", env, &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "docs\tlich\tcodex") {
		t.Errorf("stdout = %q", stdout.String())
	}
	if strings.Contains(stdout.String(), "sender") {
		t.Errorf("the caller listed itself: %q", stdout.String())
	}
}

func TestSendAndReplyOverTheRealDispatcher(t *testing.T) {
	env, term := wiredLich(t)

	var stdout, stderr bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- Run([]string{"send", "--timeout", "20", "docs", "run the tests"}, "test", env, &stdout, &stderr)
	}()

	ticketID := ticketFrom(term)
	if ticketID == "" {
		t.Fatal("the message never reached the target's terminal")
	}

	var replyOut, replyErr bytes.Buffer
	if code := Run([]string{"reply", ticketID, "3 failures"}, "test", env, &replyOut, &replyErr); code != 0 {
		t.Fatalf("reply exit = %d, stderr = %q", code, replyErr.String())
	}
	if code := <-done; code != 0 {
		t.Fatalf("send exit = %d, stderr = %q", code, stderr.String())
	}
	if strings.TrimSpace(stdout.String()) != "3 failures" {
		t.Errorf("send printed %q, want the answer the other session typed", stdout.String())
	}
}

// TestSendPrivateOverTheRealDispatcher is the whole journey of a private
// errand: the send runs out and leaves a ticket, the answer lands with nobody
// waiting, a no-ticket wait from the same session finds nothing, and the ticket
// still collects it.
func TestSendPrivateOverTheRealDispatcher(t *testing.T) {
	env, term := wiredLich(t)

	var stdout, stderr bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- Run([]string{"send", "--private", "--timeout", "1", "docs", "run the tests"}, "test", env, &stdout, &stderr)
	}()
	ticketID := ticketFrom(term)
	if ticketID == "" {
		t.Fatal("the message never reached the target's terminal")
	}
	if code := <-done; code != ExitPending {
		t.Fatalf("send exit = %d, want pending; stderr = %q", code, stderr.String())
	}

	var out, errOut bytes.Buffer
	if code := Run([]string{"reply", ticketID, "3 failures"}, "test", env, &out, &errOut); code != 0 {
		t.Fatalf("reply exit = %d, stderr = %q", code, errOut.String())
	}
	out.Reset()
	if code := Run([]string{"wait", "--json"}, "test", env, &out, &errOut); code != 0 {
		t.Fatalf("collect exit = %d, stderr = %q", code, errOut.String())
	}
	if strings.Contains(out.String(), "3 failures") {
		t.Errorf("a no-ticket wait took the private answer: %s", out.String())
	}
	out.Reset()
	if code := Run([]string{"wait", ticketID}, "test", env, &out, &errOut); code != 0 {
		t.Fatalf("wait exit = %d, stderr = %q", code, errOut.String())
	}
	if strings.TrimSpace(out.String()) != "3 failures" {
		t.Errorf("wait printed %q, want the private answer", out.String())
	}
}

// ticketFrom pulls the ticket out of the message the relay typed at the target,
// which is the only place the receiving agent ever learns it.
func ticketFrom(term *wiredTerminal) string {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		_, after, found := strings.Cut(term.message(), `"$LICH_BIN" reply `)
		if found {
			id, _, _ := strings.Cut(after, " ")
			return id
		}
		time.Sleep(time.Millisecond)
	}
	return ""
}

// spawnStore is the workspace `lich open` writes into, over the real dispatcher.
type spawnStore struct {
	mu        sync.Mutex
	rows      int
	model     string
	effort    string
	ultracode bool
	subagent  bool
	// renamed is the session id and label the last rename wrote.
	renamed [2]string
	// filed is the session id and folder the last filing wrote, and refolded
	// the project, old name and new name of the last folder rename.
	filed    [2]string
	refolded [3]string
	// confines is what the sandbox rung answers a caller with nobody to ask.
	confines bool
}

func (*spawnStore) LoadState() ([]store.Project, error) {
	return []store.Project{{ID: "p1", Name: "lich", Path: "/src/lich", NextSeq: 4, Sessions: []store.Session{
		{ID: "s1", Label: "sender", Kind: "claude"},
		{ID: "s2", Label: "auth-fix", Kind: "claude", Path: "/wt/auth-fix", Folder: "Apps"},
	}}}, nil
}

func (s *spawnStore) AddSessionFrom(_, _, _, _, _ string, _ int, _, _ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rows++
	return nil
}

func (s *spawnStore) SetSessionSubagent(string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subagent = true
	return nil
}

func (s *spawnStore) SetSessionModel(_, model string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.model = model
	return nil
}

func (s *spawnStore) SetSessionEffort(_, effort string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.effort = effort
	return nil
}

func (s *spawnStore) SetSessionUltracode(_ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ultracode = true
	return nil
}

func (s *spawnStore) SetRunEntrypoint(_, _ string) error { return nil }

func (s *spawnStore) SandboxDefault(_, _, _ string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.confines
}

func (s *spawnStore) DeleteSession(_, _, _ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rows--
	return nil
}

func (s *spawnStore) RenameSession(sessionID, label string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.renamed = [2]string{sessionID, label}
	return nil
}

func (s *spawnStore) SetSessionFolder(sessionID, folder string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.filed = [2]string{sessionID, folder}
	return nil
}

func (s *spawnStore) RenameFolder(projectID, from, to string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.refolded = [3]string{projectID, from, to}
	return []string{"s2"}, nil
}

func (*spawnStore) CloseSession(_, _, _ string) error { return nil }

func (*spawnStore) RecentProjects(string) ([]store.Recent, error) { return nil, nil }

// The wire tests never open a project: what they prove is the arguments a
// command posts, and the workspace this fixture answers with is already open.
func (*spawnStore) AddProject(_, _, _ string) error { return nil }

func (*spawnStore) PurgeWorktreeSessions(_, _ string) error { return nil }

// spawnGit stands in for the repository: creating a checkout is refused because
// the wire is what these tests prove, and running git would prove something else
// in a temporary directory. What it lists, reports dirty and records removed is
// the fixture each test sets.
type spawnGit struct {
	checkouts []project.Worktree
	dirty     bool
	removed   string
	forced    bool
}

func (*spawnGit) Branch(string) string { return "main" }

func (*spawnGit) ListBranches(string) (project.Branches, error) { return project.Branches{}, nil }

func (*spawnGit) CreateWorktree(_, _, _, _ string, _ bool) (*project.Worktree, error) {
	return nil, errors.New("no git here")
}

func (g *spawnGit) ListCheckouts(string) ([]project.Worktree, error) { return g.checkouts, nil }

func (g *spawnGit) RemoveWorktree(_, path string, force, _ bool) error {
	g.removed, g.forced = path, force
	return nil
}

func (g *spawnGit) WorktreeDirty(string) (bool, error) { return g.dirty, nil }

func (*spawnGit) WorktreeAdopted(string) bool { return false }

type spawnTerminal struct {
	mu     sync.Mutex
	kind   string
	cwd    string
	closed string
	// ran is the last command handed to a session's mod, and ranOn that
	// session.
	ran   terminal.ModCommand
	ranOn string
}

func (s *spawnTerminal) RunModCommand(
	_ context.Context, id string, cmd terminal.ModCommand,
) (terminal.ModOutcome, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ran, s.ranOn = cmd, id
	return terminal.ModOutcome{ID: "m1", State: terminal.ModAcked, OK: true}, nil
}

func (s *spawnTerminal) Start(_, _, cwd, kind, _, _ string, _, _ bool, _, _ int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cwd, s.kind = cwd, kind
	return nil
}

// Nothing renamed itself here either, so a session is addressed by the name
// lich derives for it.
func (*spawnTerminal) AgentName(string) string { return "" }

func (s *spawnTerminal) Close(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = id
	return nil
}

// wiredSpawn serves spawn behind the real RPC dispatcher and returns the session
// environment a PTY would carry, plus the workspace and the terminal it drives.
func wiredSpawn(t *testing.T, git *spawnGit) (func(string) string, *spawnStore, *spawnTerminal) {
	t.Helper()
	rows := &spawnStore{}
	term := &spawnTerminal{}
	dispatcher := rpc.New()
	dispatcher.Register("spawn", spawn.New(rows, git, term, nil))

	server := httptest.NewServer(dispatcher)
	t.Cleanup(server.Close)
	port := strconv.Itoa(server.Listener.Addr().(*net.TCPAddr).Port)
	return sessionEnv(port), rows, term
}

// TestOpenOverTheRealDispatcher proves the nine arguments `lich open` posts land
// on spawn.Open in the order it declares them — a positional mismatch here would
// otherwise open a session in a project named after a provider.
func TestOpenOverTheRealDispatcher(t *testing.T) {
	env, rows, term := wiredSpawn(t, &spawnGit{})

	var stdout, stderr bytes.Buffer
	args := []string{"open", "--kind", "codex", "--model", "gpt-5.2", "--effort", "high", "--folder", "Apps"}
	if code := Run(args, "test", env, &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"Session 4"`) {
		t.Errorf("stdout = %q, want the label the project's counter gave it", stdout.String())
	}
	if rows.rows != 1 {
		t.Errorf("wrote %d rows, want 1", rows.rows)
	}
	if term.kind != "codex" || term.cwd != "/src/lich" {
		t.Errorf("started %q in %q, want codex in the project directory", term.kind, term.cwd)
	}
	if rows.model != "gpt-5.2" {
		t.Errorf("row model = %q, want the one the flag named", rows.model)
	}
	if rows.effort != "high" {
		t.Errorf("row effort = %q, want the one the flag named", rows.effort)
	}
	if rows.filed[1] != "Apps" {
		t.Errorf("filed = %v, want the new session under the folder the flag named", rows.filed)
	}
}

// TestOpenUltracodeOverTheRealDispatcher proves the trailing boolean lands on
// spawn.Open's ultracode, the one argument the codex case above cannot carry.
func TestOpenUltracodeOverTheRealDispatcher(t *testing.T) {
	env, rows, term := wiredSpawn(t, &spawnGit{})

	var stdout, stderr bytes.Buffer
	args := []string{"open", "--kind", "claude", "--effort", "medium", "--ultracode"}
	if code := Run(args, "test", env, &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}
	if term.kind != "claude" {
		t.Errorf("started %q, want claude", term.kind)
	}
	if !rows.ultracode {
		t.Error("row ultracode = false, want the flag recorded")
	}
	if rows.effort != "medium" {
		t.Errorf("row effort = %q, want medium beside ultracode", rows.effort)
	}
}

// TestOpenSubagentOverTheRealDispatcher proves the seven arguments `lich open
// --subagent` posts land on spawn.OpenSubagent in the order it declares them.
func TestOpenSubagentOverTheRealDispatcher(t *testing.T) {
	env, rows, term := wiredSpawn(t, &spawnGit{})

	var stdout, stderr bytes.Buffer
	args := []string{
		"open", "--subagent", "--kind", "claude", "--model", "opus", "--effort", "high",
		"--ultracode", "--json", "--prompt", "port the parser",
	}
	// The hand-off has no relay behind this dispatcher, so it fails after the
	// open, which is the part under test.
	Run(args, "test", env, &stdout, &stderr)
	if rows.rows != 1 || !rows.subagent {
		t.Fatalf("rows %d, subagent %v; want one marked row (stderr %q)", rows.rows, rows.subagent, stderr.String())
	}
	if term.kind != "claude" || term.cwd != "/src/lich" {
		t.Errorf("started %q in %q, want claude in the caller's checkout", term.kind, term.cwd)
	}
	if rows.model != "opus" || rows.effort != "high" || !rows.ultracode {
		t.Errorf("row %q %q %v, want every override", rows.model, rows.effort, rows.ultracode)
	}
	if rows.filed[1] != "sender" {
		t.Errorf("filed = %v, want the caller's label", rows.filed)
	}
}

// TestCloseOverTheRealDispatcher proves the five arguments `lich close` posts
// land on spawn.Close in the order it declares them. It closes the last session
// in a dirty checkout, the case that reads every one of them: a worktree word in
// the project's slot resolves no session at all, and a --force that misses its
// own is a removal refused rather than the one the caller asked for.
func TestCloseOverTheRealDispatcher(t *testing.T) {
	git := &spawnGit{dirty: true}
	env, _, term := wiredSpawn(t, git)

	var stdout, stderr bytes.Buffer
	args := []string{"close", "--project", "lich", "--worktree", "remove", "--force", "auth-fix"}
	if code := Run(args, "test", env, &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}
	if term.closed != "s2" {
		t.Errorf("closed terminal %q, want the target session's", term.closed)
	}
	if git.removed != "/wt/auth-fix" || !git.forced {
		t.Errorf("removed %q (forced %v), want the target's checkout, forced", git.removed, git.forced)
	}
	if !strings.Contains(stdout.String(), "/wt/auth-fix") {
		t.Errorf("stdout = %q, want the checkout that went with the session", stdout.String())
	}
}

// TestRenameOverTheRealDispatcher proves the four arguments `lich rename` posts
// land on spawn.Rename in the order it declares them. Target and label are two
// strings side by side, and swapped the command renames the wrong session to the
// name of the right one.
func TestRenameOverTheRealDispatcher(t *testing.T) {
	env, rows, _ := wiredSpawn(t, &spawnGit{})

	var stdout, stderr bytes.Buffer
	args := []string{"rename", "--project", "lich", "auth-fix", "the login bug"}
	if code := Run(args, "test", env, &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}
	if rows.renamed != [2]string{"s2", "the login bug"} {
		t.Errorf("renamed = %v, want the target session under the new name", rows.renamed)
	}
	if !strings.Contains(stdout.String(), `"auth-fix" to "the login bug"`) {
		t.Errorf("stdout = %q, want both ends of the change", stdout.String())
	}
}

// TestRenameWithoutATargetRenamesTheCallersOwnSession proves the one-argument
// form reads the label rather than a session: LICH_SESSION_ID is the target, and
// a client that posted the name in the target's slot would resolve no session.
func TestRenameWithoutATargetRenamesTheCallersOwnSession(t *testing.T) {
	env, rows, _ := wiredSpawn(t, &spawnGit{})

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"rename", "planner"}, "test", env, &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}
	if rows.renamed != [2]string{"s1", "planner"} {
		t.Errorf("renamed = %v, want the calling session under the new name", rows.renamed)
	}
}

// TestControlOverTheRealDispatcher proves the six arguments `lich control`
// posts land on spawn.Control in the order it declares them, the context it
// takes first included: the action, the name and its arguments are strings side
// by side, and shifted by one the session would run the wrong command.
func TestControlOverTheRealDispatcher(t *testing.T) {
	env, _, term := wiredSpawn(t, &spawnGit{})

	var stdout, stderr bytes.Buffer
	args := []string{"control", "--project", "lich", "auth-fix", "command", "/compact", "keep the plan"}
	if code := Run(args, "test", env, &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}
	want := terminal.ModCommand{Kind: terminal.ModRunCommand, Name: "/compact", Args: "keep the plan"}
	if term.ranOn != "s2" || term.ran != want {
		t.Errorf("ran %+v on %q, want %+v on s2", term.ran, term.ranOn, want)
	}
	if stdout.String() != "\"auth-fix\" ran /compact.\n" {
		t.Errorf("stdout = %q", stdout.String())
	}
}

// TestAskOverTheRealDispatcher proves the four arguments `lich ask` posts land
// on spawn.Ask in the order it declares them: the target, the project and the
// question are strings side by side.
func TestAskOverTheRealDispatcher(t *testing.T) {
	env, _, term := wiredSpawn(t, &spawnGit{})

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"ask", "--project", "lich", "auth-fix", "why?"}, "test", env, &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}
	want := terminal.ModCommand{Kind: terminal.ModAsk, Question: "why?"}
	if term.ranOn != "s2" || term.ran != want {
		t.Errorf("ran %+v on %q, want %+v on s2", term.ran, term.ranOn, want)
	}
}

// TestWorktreesOverTheRealDispatcher proves `lich worktrees` posts the caller's
// session before the project name: swapped, the app resolves the project by a
// session id and finds none.
func TestWorktreesOverTheRealDispatcher(t *testing.T) {
	git := &spawnGit{dirty: true, checkouts: []project.Worktree{{Name: "auth-fix", Path: "/wt/auth-fix"}}}
	env, _, _ := wiredSpawn(t, git)

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"worktrees", "--project", "lich"}, "test", env, &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "auth-fix\tuncommitted\tauth-fix") {
		t.Errorf("stdout = %q, want the checkout, its state and the session in it", stdout.String())
	}
}

// TestFileOverTheRealDispatcher proves the four arguments `lich file` posts land
// on spawn.File in the order it declares them. Target and folder are two strings
// side by side, and swapped the command files the wrong session under the name
// of the right one.
func TestFileOverTheRealDispatcher(t *testing.T) {
	env, rows, _ := wiredSpawn(t, &spawnGit{})

	var stdout, stderr bytes.Buffer
	args := []string{"file", "--project", "lich", "auth-fix", "Infra"}
	if code := Run(args, "test", env, &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}
	if rows.filed != [2]string{"s2", "Infra"} {
		t.Errorf("filed = %v, want the target session under the folder", rows.filed)
	}
	if !strings.Contains(stdout.String(), `"auth-fix" under "Infra"`) {
		t.Errorf("stdout = %q, want where the session is now", stdout.String())
	}
}

// TestFileWithoutATargetFilesTheCallersOwnSession proves the one-argument form
// reads the folder rather than a session, as rename's does.
func TestFileWithoutATargetFilesTheCallersOwnSession(t *testing.T) {
	env, rows, _ := wiredSpawn(t, &spawnGit{})

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"file", "Planning"}, "test", env, &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}
	if rows.filed != [2]string{"s1", "Planning"} {
		t.Errorf("filed = %v, want the calling session under the folder", rows.filed)
	}
}

// TestFoldersOverTheRealDispatcher proves `lich folders` posts the caller's
// session before the project name, as `lich worktrees` does.
func TestFoldersOverTheRealDispatcher(t *testing.T) {
	env, _, _ := wiredSpawn(t, &spawnGit{})

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"folders", "--project", "lich"}, "test", env, &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Apps\tauth-fix") {
		t.Errorf("stdout = %q, want the folder and the session in it", stdout.String())
	}
}

// TestRenameFolderOverTheRealDispatcher proves the four arguments
// `lich rename-folder` posts land on spawn.RenameFolder in the order it declares
// them: the old name and the new one swapped would match no folder.
func TestRenameFolderOverTheRealDispatcher(t *testing.T) {
	env, rows, _ := wiredSpawn(t, &spawnGit{})

	var stdout, stderr bytes.Buffer
	args := []string{"rename-folder", "--project", "lich", "Apps", "Applications"}
	if code := Run(args, "test", env, &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}
	if rows.refolded != [3]string{"p1", "Apps", "Applications"} {
		t.Errorf("refolded = %v, want the project's folder under the new name", rows.refolded)
	}
	if !strings.Contains(stdout.String(), `"auth-fix" from folder "Apps" to "Applications"`) {
		t.Errorf("stdout = %q, want every session that moved", stdout.String())
	}
}
