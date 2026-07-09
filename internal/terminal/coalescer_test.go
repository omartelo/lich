package terminal

import (
	"bytes"
	"sync"
	"testing"
	"time"
)

// captureEmit returns an emit func that copies every emission into a channel.
func captureEmit(buf int) (func([]byte), chan []byte) {
	emits := make(chan []byte, buf)
	return func(data []byte) {
		emits <- append([]byte(nil), data...)
	}, emits
}

func TestCoalescerVisibleEmitsImmediately(t *testing.T) {
	emit, emits := captureEmit(1)
	c := newCoalescer(emit, time.Hour)

	c.Write([]byte("hello"))

	select {
	case got := <-emits:
		if string(got) != "hello" {
			t.Fatalf("emitted %q, want %q", got, "hello")
		}
	default:
		t.Fatal("visible write did not emit immediately")
	}
}

func TestCoalescerHiddenBuffersThenFlushesOnTimer(t *testing.T) {
	emit, emits := captureEmit(1)
	c := newCoalescer(emit, 10*time.Millisecond)
	c.SetVisible(false)

	c.Write([]byte("foo"))
	c.Write([]byte("bar"))

	select {
	case got := <-emits:
		t.Fatalf("hidden write emitted %q before the flush interval", got)
	default:
	}

	select {
	case got := <-emits:
		if string(got) != "foobar" {
			t.Fatalf("flushed %q, want %q", got, "foobar")
		}
	case <-time.After(time.Second):
		t.Fatal("timer flush never happened")
	}
}

func TestCoalescerTimerRearmsPerBurst(t *testing.T) {
	emit, emits := captureEmit(2)
	c := newCoalescer(emit, 10*time.Millisecond)
	c.SetVisible(false)

	c.Write([]byte("first"))
	if got := <-emits; string(got) != "first" {
		t.Fatalf("first flush = %q", got)
	}

	c.Write([]byte("second"))
	select {
	case got := <-emits:
		if string(got) != "second" {
			t.Fatalf("second flush = %q", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timer did not re-arm for the second burst")
	}
}

func TestCoalescerSetVisibleFlushesPending(t *testing.T) {
	emit, emits := captureEmit(1)
	c := newCoalescer(emit, time.Hour)
	c.SetVisible(false)

	c.Write([]byte("pending"))
	c.SetVisible(true)

	select {
	case got := <-emits:
		if string(got) != "pending" {
			t.Fatalf("flushed %q, want %q", got, "pending")
		}
	default:
		t.Fatal("SetVisible(true) did not flush pending output")
	}
}

func TestCoalescerSetVisibleWithEmptyPendingEmitsNothing(t *testing.T) {
	emit, emits := captureEmit(1)
	c := newCoalescer(emit, time.Hour)
	c.SetVisible(false)

	c.SetVisible(true)

	select {
	case got := <-emits:
		t.Fatalf("empty flush emitted %q", got)
	default:
	}
}

func TestCoalescerOverflowForcesEarlyFlush(t *testing.T) {
	emit, emits := captureEmit(1)
	c := newCoalescer(emit, time.Hour)
	c.SetVisible(false)

	c.Write(make([]byte, maxPendingBytes))

	select {
	case got := <-emits:
		if len(got) != maxPendingBytes {
			t.Fatalf("flushed %d bytes, want %d", len(got), maxPendingBytes)
		}
	default:
		t.Fatal("overflow did not force a flush")
	}
}

func TestCoalescerCloseFlushesOnceAndSeals(t *testing.T) {
	emit, emits := captureEmit(2)
	c := newCoalescer(emit, time.Hour)
	c.SetVisible(false)

	c.Write([]byte("tail"))
	c.Close()
	c.Close()
	c.Write([]byte("dropped"))

	if got := <-emits; string(got) != "tail" {
		t.Fatalf("close flushed %q, want %q", got, "tail")
	}
	select {
	case got := <-emits:
		t.Fatalf("unexpected emission after close: %q", got)
	default:
	}
}

// TestCoalescerConcurrentWritesAndFlips hammers Write and SetVisible from
// multiple goroutines; under -race this checks the locking, and afterwards the
// total emitted bytes must equal the total written.
func TestCoalescerConcurrentWritesAndFlips(t *testing.T) {
	var mu sync.Mutex
	var emitted bytes.Buffer
	c := newCoalescer(func(data []byte) {
		mu.Lock()
		emitted.Write(data)
		mu.Unlock()
	}, time.Millisecond)

	const writers, writes = 4, 100
	var wg sync.WaitGroup
	for range writers {
		wg.Go(func() {
			for range writes {
				c.Write([]byte("x"))
			}
		})
	}
	wg.Go(func() {
		for i := range 50 {
			c.SetVisible(i%2 == 0)
		}
	})
	wg.Wait()
	c.SetVisible(true) // final flush of anything still pending
	c.Close()

	mu.Lock()
	defer mu.Unlock()
	if got, want := emitted.Len(), writers*writes; got != want {
		t.Fatalf("emitted %d bytes, want %d", got, want)
	}
}
