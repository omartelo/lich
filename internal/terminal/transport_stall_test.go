package terminal

import (
	"sync/atomic"
	"testing"
	"time"
)

// stallBudget bounds how long a send for one session may take while another
// session's output is stuck in the socket. The work send has to do is encode a
// frame and hand it over, so the real number is microseconds; the budget only
// has to sit far below wsWriteTimeout, which is what a write the senders queue
// behind costs when one lock serializes them all.
const stallBudget = time.Second

// stallFrameBytes and stallFrames are the load that fills the client's socket:
// a non-reading peer buffers a few megabytes between the two kernels before a
// write blocks, so this overshoots that on purpose. Pinned rather than derived
// from writeQueueDepth — the point is that the frames outlast the socket and
// still leave the transport room to take one more.
const (
	stallFrameBytes = 1 << 20
	stallFrames     = 24
)

// TestStalledSessionDoesNotBlockAnother is the regression for a freeze that
// outbox.go alone could not prevent. Each session gets its own queue and its own
// delivery goroutine so a window that stops reading backs up the session that is
// producing and nobody else — but every one of those goroutines went through
// transport.send, which held the transport's mutex across conn.Write. One
// stalled write, bounded only by wsWriteTimeout, therefore parked every session
// behind it: the per-session decoupling, undone one layer down.
//
// The client here connects and never reads, so the loud session's frames fill
// the socket and stay there. A quiet session's send must still return promptly
// — it is handing a frame to the connection's writer, not waiting on the wire.
func TestStalledSessionDoesNotBlockAnother(t *testing.T) {
	tr, err := newTransport(func(string, []byte) {}, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("newTransport: %v", err)
	}

	// Never read from: a client that drains the socket is a client that never
	// stalls the write, and there would be nothing to measure.
	client, err := dial(t, tr, tr.token)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = client.CloseNow() }()
	waitForClient(t, tr)

	var accepted atomic.Int64
	loud := make(chan struct{})
	go func() {
		defer close(loud)
		payload := make([]byte, stallFrameBytes)
		for i := 0; i < stallFrames; i++ {
			tr.send("loud", payload)
			accepted.Add(1)
		}
	}()

	waitForStall(t, &accepted, loud)

	start := time.Now()
	tr.send("quiet", []byte("still here"))
	blocked := time.Since(start)
	t.Logf("a send for an idle session waited %v on a stalled one", blocked.Round(time.Millisecond))
	if blocked > stallBudget {
		t.Fatalf("a send for an idle session waited %v on a stalled one, want under %v",
			blocked.Round(time.Millisecond), stallBudget)
	}

	// The loud session is still stuck in the socket; it comes back when the send
	// timeout drops the connection. Waiting keeps the goroutine out of the tests
	// that run after this one.
	select {
	case <-loud:
	case <-time.After(2 * wsWriteTimeout):
		t.Fatal("the stalled session never came back")
	}
}

// waitForStall blocks until the loud session stops making progress and the
// socket has had a moment to fill.
//
// Progress stops in one of two shapes, and the assertion after it is the same
// either way. A transport that writes on the caller's goroutine stops because
// the send that filled the socket never returns. A transport that queues stops
// because every frame was accepted and the goroutine finished — the writer is
// then alone against a peer that is not reading, which is the state the
// assertion needs, so the settle below gives it time to hit that wall.
func waitForStall(t *testing.T, accepted *atomic.Int64, done <-chan struct{}) {
	t.Helper()
	const (
		quietFor = 250 * time.Millisecond
		settle   = 250 * time.Millisecond
	)
	deadline := time.Now().Add(10 * time.Second)
	last := int64(-1)
	still := time.Now()
	for time.Now().Before(deadline) {
		if n := accepted.Load(); n != last {
			last, still = n, time.Now()
		}
		if last > 0 && time.Since(still) > quietFor {
			time.Sleep(settle)
			return
		}
		select {
		case <-done:
			time.Sleep(settle)
			return
		default:
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("the loud session never filled the socket")
}
