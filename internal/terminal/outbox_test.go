package terminal

import (
	"sync"
	"testing"
	"time"
)

// TestOutboxQueuesWhileDeliveryIsStuck is the regression for the freeze a large
// resumed session caused: with delivery blocked (a window that stopped reading
// the socket), pushes must keep returning until the queue is full, so the PTY
// read loop behind them keeps draining and only this session — never its
// neighbours — feels the backpressure.
func TestOutboxQueuesWhileDeliveryIsStuck(t *testing.T) {
	release := make(chan struct{})
	entered := make(chan struct{}, 1)
	box := newOutbox(func([]byte) {
		select {
		case entered <- struct{}{}:
		default:
		}
		<-release
	}, 2)

	// The first frame is taken by the (stuck) delivery goroutine; two more fit
	// the queue.
	pushed := make(chan struct{})
	go func() {
		box.push([]byte("one"))
		box.push([]byte("two"))
		box.push([]byte("three"))
		close(pushed)
	}()
	<-entered
	select {
	case <-pushed:
	case <-time.After(2 * time.Second):
		t.Fatal("push blocked while the queue still had room")
	}

	// A fourth has nowhere to go: this session's producer waits, which is the
	// backpressure we keep.
	blocked := make(chan struct{})
	go func() {
		box.push([]byte("four"))
		close(blocked)
	}()
	select {
	case <-blocked:
		t.Fatal("push past the queue depth did not block")
	case <-time.After(50 * time.Millisecond):
	}

	close(release)
	<-blocked
	box.close()
}

// TestOutboxDeliversInOrderAndDrainsOnClose proves close waits for the frames
// already queued: the exit event a caller emits after it can never overtake the
// session's last bytes.
func TestOutboxDeliversInOrderAndDrainsOnClose(t *testing.T) {
	var mu sync.Mutex
	var got []string
	box := newOutbox(func(data []byte) {
		mu.Lock()
		got = append(got, string(data))
		mu.Unlock()
	}, 4)

	for _, frame := range []string{"a", "b", "c", "d", "e"} {
		box.push([]byte(frame))
	}
	box.close()

	mu.Lock()
	defer mu.Unlock()
	if want := "abcde"; join(got) != want {
		t.Errorf("delivered %q, want %q", join(got), want)
	}
}

// TestOutboxCopiesTheFrame proves a caller may reuse its buffer after push —
// the coalescer hands over the slice it keeps appending to.
func TestOutboxCopiesTheFrame(t *testing.T) {
	delivered := make(chan []byte, 1)
	box := newOutbox(func(data []byte) { delivered <- data }, 1)

	buf := []byte("live")
	box.push(buf)
	copy(buf, "dead")
	box.close()

	if got := string(<-delivered); got != "live" {
		t.Errorf("delivered %q, want %q — the frame was not copied", got, "live")
	}
}

func join(frames []string) string {
	out := ""
	for _, frame := range frames {
		out += frame
	}
	return out
}
