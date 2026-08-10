package cli

import (
	"bytes"
	"net"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/omartelo/lich/internal/relay"
	"github.com/omartelo/lich/internal/rpc"
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
	if code := Run([]string{"sessions"}, env, &stdout, &stderr); code != 0 {
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
		done <- Run([]string{"send", "--timeout", "20", "docs", "run the tests"}, env, &stdout, &stderr)
	}()

	ticketID := ticketFrom(term)
	if ticketID == "" {
		t.Fatal("the message never reached the target's terminal")
	}

	var replyOut, replyErr bytes.Buffer
	if code := Run([]string{"reply", ticketID, "3 failures"}, env, &replyOut, &replyErr); code != 0 {
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
