package terminal

import (
	"context"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// TestReplacedClientDoesNotSilenceTransport models a page reload: a second /ws
// socket opens, the transport replaces the first, and the first's read loop
// then drops the connection it was reading. The live client must keep receiving
// — forgetting the replaced connection must not forget the current one. A
// transport that forgets it has a client sends every session's output down the
// /events fallback instead, for a socket that is still open.
func TestReplacedClientDoesNotSilenceTransport(t *testing.T) {
	tr, err := newTransport(func(string, []byte) {}, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("newTransport: %v", err)
	}

	first, err := dial(t, tr, tr.token)
	if err != nil {
		t.Fatalf("dial first: %v", err)
	}
	defer func() { _ = first.CloseNow() }()
	replaced := waitForClient(t, tr)

	second, err := dial(t, tr, tr.token)
	if err != nil {
		t.Fatalf("dial second: %v", err)
	}
	defer func() { _ = second.CloseNow() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// handle swaps the transport's connection before it closes the one it
	// replaced, so the first client's read failing proves the second one is
	// already the transport's client. Polling t.conn would be the worse signal:
	// without the identity guard the read loop nils it out, and the wait would
	// fail on its own before the assertion below ever ran. This read is also
	// what answers the close handshake — handle's close waits for the reply.
	if _, _, err := first.Read(ctx); err == nil {
		t.Fatal("the transport never closed the replaced client")
	}

	// The replaced connection's read loop calls drop on its own the moment it
	// notices handle closed it; calling drop here asserts on that outcome
	// instead of racing the goroutine for it. Under the identity guard the
	// second call is the no-op the first one already was.
	tr.drop(replaced)

	if !tr.send("sess", []byte("output")) {
		t.Fatal("send reported no client after the replaced one was dropped")
	}
	kind, data, err := second.Read(ctx)
	if err != nil {
		t.Fatalf("live client received nothing after the replaced one was dropped: %v", err)
	}
	if kind != websocket.MessageBinary {
		t.Fatalf("got message type %v", kind)
	}
	id, payload, err := decodeFrame(data)
	if err != nil {
		t.Fatalf("decodeFrame: %v", err)
	}
	if id != "sess" || string(payload) != "output" {
		t.Fatalf("got id=%q payload=%q", id, payload)
	}
}

// waitForClient blocks until the transport has registered a client, and returns
// it. Dial returning only means the client read the handshake response; handle
// registers the connection in its own goroutine.
func waitForClient(t *testing.T, tr *transport) *websocket.Conn {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		tr.mu.Lock()
		current := tr.conn
		tr.mu.Unlock()
		if current != nil {
			return current
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("transport never registered a client")
	return nil
}
