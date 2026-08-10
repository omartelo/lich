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
	mu   sync.Mutex
	rows int
}

func (*spawnStore) LoadState() ([]store.Project, error) {
	return []store.Project{{ID: "p1", Name: "lich", Path: "/src/lich", NextSeq: 4,
		Sessions: []store.Session{{ID: "s1", Label: "sender", Kind: "claude"}}}}, nil
}

func (s *spawnStore) AddSession(_, _, _, _, _ string, _ int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rows++
	return nil
}

// spawnGit refuses every worktree: the wire is what these tests prove, and
// running git would prove something else in a temporary directory.
type spawnGit struct{}

func (spawnGit) Branch(string) string { return "main" }

func (spawnGit) ListBranches(string) (project.Branches, error) { return project.Branches{}, nil }

func (spawnGit) CreateWorktree(_, _, _, _ string, _ bool) (*project.Worktree, error) {
	return nil, errors.New("no git here")
}

type spawnTerminal struct {
	mu   sync.Mutex
	kind string
	cwd  string
}

func (s *spawnTerminal) Start(_, _, cwd, kind, _, _ string, _ bool, _, _ int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cwd, s.kind = cwd, kind
	return nil
}

// TestOpenOverTheRealDispatcher proves the five arguments `lich open` posts land
// on spawn.Open in the order it declares them — a positional mismatch here would
// otherwise open a session in a project named after a provider.
func TestOpenOverTheRealDispatcher(t *testing.T) {
	rows := &spawnStore{}
	term := &spawnTerminal{}
	dispatcher := rpc.New()
	dispatcher.Register("spawn", spawn.New(rows, spawnGit{}, term, nil))

	server := httptest.NewServer(dispatcher)
	t.Cleanup(server.Close)
	port := strconv.Itoa(server.Listener.Addr().(*net.TCPAddr).Port)
	env := func(key string) string {
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

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"open", "--kind", "codex"}, "test", env, &stdout, &stderr); code != 0 {
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
}
