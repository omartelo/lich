package cli

import (
	"bytes"
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

type wiredTerminal struct {
	mu    sync.Mutex
	typed string
}

// Pointer receiver like Write's: a value receiver would copy the mutex beside
// it on every call, which is a race the moment the two are used together.
func (*wiredTerminal) Live(string) bool { return true }

func (*wiredTerminal) Ready(string) bool { return true }

func (*wiredTerminal) QuietFor(string) time.Duration { return time.Hour }

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
	}, term
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
	mu    sync.Mutex
	rows  int
	model string
	// renamed is the session id and label the last rename wrote.
	renamed [2]string
}

func (*spawnStore) LoadState() ([]store.Project, error) {
	return []store.Project{{ID: "p1", Name: "lich", Path: "/src/lich", NextSeq: 4, Sessions: []store.Session{
		{ID: "s1", Label: "sender", Kind: "claude"},
		{ID: "s2", Label: "auth-fix", Kind: "claude", Path: "/wt/auth-fix"},
	}}}, nil
}

func (s *spawnStore) AddSessionFrom(_, _, _, _, _ string, _ int, _, _ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rows++
	return nil
}

func (s *spawnStore) SetSessionModel(_, model string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.model = model
	return nil
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

func (*spawnStore) CloseSession(_, _, _ string) error { return nil }

func (*spawnStore) RecentProjects() ([]store.Recent, error) { return nil, nil }

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

func (g *spawnGit) RemoveWorktree(_, path string, force bool) error {
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
}

func (s *spawnTerminal) Start(_, _, cwd, kind, _, _ string, _ bool, _, _ int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cwd, s.kind = cwd, kind
	return nil
}

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
	}, rows, term
}

// TestOpenOverTheRealDispatcher proves the six arguments `lich open` posts land
// on spawn.Open in the order it declares them — a positional mismatch here would
// otherwise open a session in a project named after a provider.
func TestOpenOverTheRealDispatcher(t *testing.T) {
	env, rows, term := wiredSpawn(t, &spawnGit{})

	var stdout, stderr bytes.Buffer
	args := []string{"open", "--kind", "codex", "--model", "gpt-5.2"}
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
