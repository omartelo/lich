package cli

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/omartelo/lich/internal/relay"
	"github.com/omartelo/lich/internal/rpc"
)

// These prove that a caller who stops waiting for results stops taking them. A
// collect nobody reads any more used to stay registered in the relay, so the
// next result woke it instead of nudging the sender, was drained into a reply
// nobody read, and was gone.

// collectGate reports when a relay.Collect request enters and leaves the
// dispatcher, which is how a test knows the wait is being held and whether the
// backend let go of it.
type collectGate struct {
	entered chan struct{}
	left    chan struct{}
}

func wiredRelay(t *testing.T) (func(string) string, *wiredTerminal, *relay.Service, collectGate) {
	t.Helper()
	term := &wiredTerminal{}
	svc := relay.New(wiredSessions{}, term, nil)
	dispatcher := rpc.New()
	dispatcher.Register("relay", svc)
	gate := collectGate{entered: make(chan struct{}, 8), left: make(chan struct{}, 8)}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		collecting := strings.HasSuffix(r.URL.Path, "relay.Collect")
		if collecting {
			gate.entered <- struct{}{}
		}
		dispatcher.ServeHTTP(w, r)
		if collecting {
			gate.left <- struct{}{}
		}
	}))
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
	return env, term, svc, gate
}

// openErrand leaves one errand from s1 open and unattended, and returns its
// ticket: a collect only holds the line while something is still owed.
func openErrand(t *testing.T, env func(string) string, term *wiredTerminal) string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"send", "--timeout", "1", "docs", "run the tests"}, "test", env, &stdout, &stderr); code != ExitPending {
		t.Fatalf("send exit = %d, want the wait to run out: %q", code, stderr.String())
	}
	ticket := ticketFrom(term)
	if ticket == "" {
		t.Fatal("the message never reached the target's terminal")
	}
	return ticket
}

// awaitSignal reports whether ch fires within a short bound.
func awaitSignal(ch <-chan struct{}, bound time.Duration) bool {
	select {
	case <-ch:
		return true
	case <-time.After(bound):
		return false
	}
}

// answerAndCheckTheInbox replies to the errand once the abandoned collect has
// had every chance to take the answer, then proves the answer is still there
// for the sender.
func answerAndCheckTheInbox(t *testing.T, env func(string) string, svc *relay.Service, gate collectGate, ticket string) {
	t.Helper()
	var out, errOut bytes.Buffer
	if code := Run([]string{"reply", ticket, "3 failures"}, "test", env, &out, &errOut); code != 0 {
		t.Fatalf("reply exit = %d, stderr = %q", code, errOut.String())
	}
	// A collect still held after the hang-up takes the answer as soon as it
	// lands; its request finishing is the sign it is done taking.
	awaitSignal(gate.left, time.Second)

	collected, err := svc.CollectNow("s1")
	if err != nil {
		t.Fatalf("CollectNow: %v", err)
	}
	if len(collected.Results) != 1 || collected.Results[0].Answer != "3 failures" {
		t.Fatalf("inbox holds %+v, want the answer the abandoned wait must not have taken", collected)
	}
}

func TestAWaitKilledMidCollectLeavesTheResultInTheInbox(t *testing.T) {
	env, term, svc, gate := wiredRelay(t)
	ticket := openErrand(t, env, term)

	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		url := endpoint(env("LICH_PORT"), env("LICH_TOKEN"), "relay.Collect")
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(`["s1", 30]`))
		if err != nil {
			t.Errorf("build the request: %v", err)
			return
		}
		if resp, err := http.DefaultClient.Do(request); err == nil {
			_ = resp.Body.Close()
		}
	}()
	if !awaitSignal(gate.entered, 2*time.Second) {
		t.Fatal("the collect never reached the dispatcher")
	}
	// What Ctrl-C does to `lich wait`: the process dies and its connection with it.
	cancel()
	<-finished
	if !awaitSignal(gate.left, time.Second) {
		t.Error("the backend kept holding a collect whose caller hung up")
	}

	answerAndCheckTheInbox(t, env, svc, gate, ticket)
}

func TestACancelledWaitForAnswerLeavesTheResultInTheInbox(t *testing.T) {
	env, term, svc, gate := wiredRelay(t)
	ticket := openErrand(t, env, term)

	stdinRead, stdinWrite := io.Pipe()
	var stdout lockedBuffer
	c := &client{env: env, stdin: stdinRead, stdout: &stdout, stderr: io.Discard, running: noInstance}
	served := make(chan int, 1)
	go func() { served <- dispatch([]string{"mcp"}, c) }()

	// Written from its own goroutine, as a client's writer is: a server that stops
	// reading while a call runs must not also stall the test that proves it.
	lines := make(chan string, 4)
	go func() {
		for line := range lines {
			_, _ = io.WriteString(stdinWrite, line+"\n")
		}
		_ = stdinWrite.Close()
	}()
	send := func(line string) { lines <- line }
	send(`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"wait_for_answer","arguments":{"timeout_seconds":10}}}`)
	if !awaitSignal(gate.entered, 2*time.Second) {
		t.Fatal("the tool call never reached the dispatcher")
	}
	// Anything else the client sends meanwhile is still answered, after the call.
	send(`{"jsonrpc":"2.0","id":8,"method":"ping"}`)
	// What Esc does in a client that implements cancellation.
	send(`{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":7,"reason":"user interrupted"}}`)
	if !awaitSignal(gate.left, time.Second) {
		t.Error("the cancellation never reached the backend's wait")
	}

	answerAndCheckTheInbox(t, env, svc, gate, ticket)

	close(lines)
	if code := <-served; code != 0 {
		t.Fatalf("mcp exit = %d", code)
	}
	if got := stdout.String(); got != `{"jsonrpc":"2.0","id":8,"result":{}}`+"\n" {
		t.Errorf("stdout = %q, want the ping answered and the cancelled call not", got)
	}
}

// lockedBuffer is a bytes.Buffer the server may write while the test reads it.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
