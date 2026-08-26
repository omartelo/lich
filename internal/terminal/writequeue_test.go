package terminal

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// frameFor encodes one session frame the way send does.
func frameFor(t *testing.T, id, payload string) []byte {
	t.Helper()
	frame, err := encodeFrame(id, []byte(payload))
	if err != nil {
		t.Fatalf("encodeFrame: %v", err)
	}
	return frame
}

// TestWriteQueueBlocksOnlyOnceFull pins the backpressure: pushes return while
// the queue has room, so the outbox goroutine behind them is free to carry on,
// and only a full queue makes a session wait — for the socket it is filling,
// never for another session's.
func TestWriteQueueBlocksOnlyOnceFull(t *testing.T) {
	q := newWriteQueue()

	filled := make(chan struct{})
	go func() {
		defer close(filled)
		for i := 0; i < writeQueueDepth; i++ {
			q.push(frameFor(t, "sess", "x"))
		}
	}()
	select {
	case <-filled:
	case <-time.After(2 * time.Second):
		t.Fatal("a push blocked while the queue still had room")
	}

	blocked := make(chan struct{})
	go func() {
		q.push(frameFor(t, "sess", "over"))
		close(blocked)
	}()
	select {
	case <-blocked:
		t.Fatal("a push past the queue depth did not block")
	case <-time.After(50 * time.Millisecond):
	}

	// Taking one frame off leaves room for the frame that was waiting.
	if _, ok := q.pop(); !ok {
		t.Fatal("pop found nothing in a full queue")
	}
	q.settle()
	select {
	case <-blocked:
	case <-time.After(2 * time.Second):
		t.Fatal("a push stayed blocked after the queue made room")
	}
}

// TestWriteQueueDrainsBeforeRejectingAPush is the ordering guarantee across the
// switch to the fallback: once the queue is closed its caller delivers frames
// itself, so a push may only report failure after the frames queued ahead of it
// are out — otherwise the newest frame would reach the client first.
func TestWriteQueueDrainsBeforeRejectingAPush(t *testing.T) {
	q := newWriteQueue()
	q.push(frameFor(t, "sess", "a"))
	q.push(frameFor(t, "sess", "b"))
	q.close()

	rejected := make(chan struct{})
	go func() {
		defer close(rejected)
		if q.push(frameFor(t, "sess", "c")) {
			t.Error("a closed queue accepted a frame")
		}
	}()
	select {
	case <-rejected:
		t.Fatal("a push reported failure before the frames ahead of it were drained")
	case <-time.After(50 * time.Millisecond):
	}

	// What the writer does with the rest of a closed queue.
	var got []string
	for {
		frame, ok := q.pop()
		if !ok {
			break
		}
		_, data, err := decodeFrame(frame)
		if err != nil {
			t.Fatalf("decodeFrame: %v", err)
		}
		got = append(got, string(data))
		q.settle()
	}
	if want := "ab"; join(got) != want {
		t.Errorf("drained %q, want %q", join(got), want)
	}

	select {
	case <-rejected:
	case <-time.After(2 * time.Second):
		t.Fatal("a push never came back after the queue drained")
	}
}

// TestWriteQueueFlushWaitsForTheFrameInTheWritersHands proves flush covers the
// write itself and not just the wait for one: a session emits its exit event
// after flush returns, and a frame the writer has taken but not yet put on the
// socket would otherwise be overtaken by the banner.
func TestWriteQueueFlushWaitsForTheFrameInTheWritersHands(t *testing.T) {
	q := newWriteQueue()
	q.push(frameFor(t, "sess", "last"))

	if _, ok := q.pop(); !ok {
		t.Fatal("pop found nothing")
	}

	flushed := make(chan struct{})
	go func() {
		q.flush()
		close(flushed)
	}()
	select {
	case <-flushed:
		t.Fatal("flush returned while the writer still held a frame")
	case <-time.After(50 * time.Millisecond):
	}

	q.settle()
	select {
	case <-flushed:
	case <-time.After(2 * time.Second):
		t.Fatal("flush never returned after the writer finished")
	}
}

// TestWriteQueueTakesRefusedFramesToTheFallback pins the outbox contract one
// layer down: output is never lost. A socket that refuses a frame costs the
// connection, not the bytes — that frame and every one queued behind it go to
// the /events bridge instead, in the order the sessions produced them, and the
// transport is told once that the client is gone.
func TestWriteQueueTakesRefusedFramesToTheFallback(t *testing.T) {
	tr, err := newTransport(func(string, []byte) {}, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("newTransport: %v", err)
	}
	client, err := dial(t, tr, tr.token)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = client.CloseNow() }()

	// A connection that is already gone refuses the first write, which is the
	// failure without the wait for one.
	conn := waitForClient(t, tr)
	_ = conn.CloseNow()

	q := newWriteQueue()
	for _, payload := range []string{"a", "b", "c"} {
		q.push(frameFor(t, "sess", payload))
	}
	q.close()

	var mu sync.Mutex
	var got []string
	var failures atomic.Int64
	done := make(chan struct{})
	go func() {
		defer close(done)
		q.run(conn, func() { failures.Add(1) }, func(id string, data []byte) {
			mu.Lock()
			got = append(got, id+":"+string(data))
			mu.Unlock()
		})
	}()
	select {
	case <-done:
	case <-time.After(2 * wsWriteTimeout):
		t.Fatal("the writer never finished with a dead socket")
	}

	mu.Lock()
	defer mu.Unlock()
	if want := "sess:asess:bsess:c"; join(got) != want {
		t.Errorf("the fallback got %q, want %q", join(got), want)
	}
	if n := failures.Load(); n != 1 {
		t.Errorf("the transport was told the client was gone %d times, want 1", n)
	}
}
