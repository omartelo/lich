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

// TestCoalescerVisibleBatchesBurstOnShortTimer is the trailing half of the
// contract: everything written inside the window behind the leading write
// batches into a single emission. It pairs a short visible interval with an
// hour-long hidden one, so the flush also proves the timer picked the visible
// cadence.
func TestCoalescerVisibleBatchesBurstOnShortTimer(t *testing.T) {
	emit, emits := captureEmit(3)
	c := newCoalescer(emit, 20*time.Millisecond, time.Hour)

	c.Write([]byte("h"))
	if got := <-emits; string(got) != "h" {
		t.Fatalf("leading flush = %q, want %q", got, "h")
	}

	for _, chunk := range []string{"e", "l", "lo"} {
		c.Write([]byte(chunk))
		select {
		case got := <-emits:
			t.Fatalf("burst write emitted %q before the flush interval", got)
		default:
		}
	}

	select {
	case got := <-emits:
		if string(got) != "ello" {
			t.Fatalf("flushed %q, want %q", got, "ello")
		}
	case <-time.After(time.Second):
		t.Fatal("visible timer flush never happened")
	}
	select {
	case got := <-emits:
		t.Fatalf("burst emitted twice, second was %q", got)
	default:
	}
}

// TestCoalescerVisibleLeadingEdgeEmitsIdleWriteAtOnce pins the leading edge: a
// lone write on an idle visible terminal — a keystroke echo — has no burst to
// collapse, so it must not pay the window.
func TestCoalescerVisibleLeadingEdgeEmitsIdleWriteAtOnce(t *testing.T) {
	emit, emits := captureEmit(1)
	c := newCoalescer(emit, time.Hour, time.Hour)

	c.Write([]byte("k"))

	select {
	case got := <-emits:
		if string(got) != "k" {
			t.Fatalf("leading flush = %q, want %q", got, "k")
		}
	default:
		t.Fatal("idle write waited for the flush interval instead of emitting at once")
	}
}

// TestCoalescerLeadingEdgeWindowBoundary pins both sides of the idle check: a
// write a whole window after the last emission is leading, one still inside
// the window is not.
func TestCoalescerLeadingEdgeWindowBoundary(t *testing.T) {
	const window = 20 * time.Millisecond

	atEdge, atEdgeEmits := captureEmit(1)
	edge := newCoalescer(atEdge, window, time.Hour)
	edge.lastEmit = time.Now().Add(-window)

	edge.Write([]byte("edge"))

	select {
	case got := <-atEdgeEmits:
		if string(got) != "edge" {
			t.Fatalf("edge flush = %q, want %q", got, "edge")
		}
	default:
		t.Fatal("a write exactly one window after the last emission did not take the leading edge")
	}

	// An hour-long window keeps the inside-the-window side immune to
	// scheduling jitter: this write is a whole hour short of the edge.
	inWindow, inWindowEmits := captureEmit(1)
	inside := newCoalescer(inWindow, time.Hour, time.Hour)
	inside.lastEmit = time.Now()

	inside.Write([]byte("early"))

	select {
	case got := <-inWindowEmits:
		t.Fatalf("a write inside the window took the leading edge and emitted %q", got)
	default:
	}
}

// TestCoalescerHiddenNeverTakesLeadingEdge: a hidden terminal does not paint,
// so it has no echo latency to protect and batches hard instead.
func TestCoalescerHiddenNeverTakesLeadingEdge(t *testing.T) {
	emit, emits := captureEmit(1)
	c := newCoalescer(emit, time.Hour, time.Hour)
	c.SetVisible(false)

	c.Write([]byte("bg"))

	select {
	case got := <-emits:
		t.Fatalf("hidden write took the leading edge and emitted %q", got)
	default:
	}
}

// TestCoalescerHiddenBuffersThenFlushesOnTimer is the mirror image: an
// hour-long visible interval with a short hidden one.
func TestCoalescerHiddenBuffersThenFlushesOnTimer(t *testing.T) {
	emit, emits := captureEmit(1)
	c := newCoalescer(emit, time.Hour, 10*time.Millisecond)
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
	c := newCoalescer(emit, time.Hour, 10*time.Millisecond)
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
	c := newCoalescer(emit, time.Hour, time.Hour)
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
	c := newCoalescer(emit, time.Hour, time.Hour)
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
	c := newCoalescer(emit, time.Hour, time.Hour)
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
	c := newCoalescer(emit, time.Hour, time.Hour)
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
	}, time.Millisecond, time.Millisecond)

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

// BenchmarkEchoLatency measures what a keystroke echo pays: the wall time from
// a lone write on an idle visible coalescer to its emission, at the cadence
// production runs.
func BenchmarkEchoLatency(b *testing.B) {
	emitted := make(chan time.Time, 1)
	c := newCoalescer(func([]byte) { emitted <- time.Now() }, visibleFlushInterval, hiddenFlushInterval)
	defer c.Close()

	var total time.Duration
	samples := 0
	for b.Loop() {
		time.Sleep(visibleFlushInterval) // lapse the window: the next write is a fresh echo
		start := time.Now()
		c.Write([]byte("x"))
		total += (<-emitted).Sub(start)
		samples++
	}
	b.ReportMetric(float64(total.Microseconds())/float64(samples), "us/echo")
}
