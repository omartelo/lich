package terminal

import (
	"sync"
	"time"
)

const (
	// visibleFlushInterval batches a visible session's output just under one
	// 60Hz frame: a burst of tiny PTY reads (an nvim redraw) collapses into a
	// couple of events instead of dozens, each paying the event-bridge
	// overhead (IPC + base64 + WASM parse), while echo latency stays
	// imperceptible.
	visibleFlushInterval = 8 * time.Millisecond
	// hiddenFlushInterval is how often a hidden session's buffered output is
	// flushed to the frontend. Hidden terminals don't paint, so latency is
	// irrelevant; batching cuts the event-bridge overhead to a few events per
	// second.
	hiddenFlushInterval = 250 * time.Millisecond
	// maxPendingBytes forces an early flush, bounding both the buffer's
	// memory and the size of a single frontend event.
	maxPendingBytes = 256 * 1024
)

// coalescer batches PTY output before it is emitted to the frontend, on a
// short cadence while visible and a long one while hidden. It is leading +
// trailing while visible: the first write after a silent window goes out at
// once, and the timer then batches whatever the burst adds behind it. emit is
// called with the mutex held, so emissions are strictly ordered; emit must not
// retain the slice.
type coalescer struct {
	mu           sync.Mutex
	emit         func(data []byte)
	visibleEvery time.Duration
	hiddenEvery  time.Duration
	visible      bool
	pending      []byte
	// timer is armed (one-shot) only while data is pending, so an idle
	// session costs nothing.
	timer *time.Timer
	// lastEmit dates the previous emission, so a write can tell an idle
	// terminal (nothing to batch, take the leading edge) from one still
	// inside a burst's window. Its zero value makes the first write leading.
	lastEmit time.Time
	closed   bool
}

// newCoalescer returns a coalescer that starts visible — sessions spawn lazily
// on first view, so a session is always on screen when created.
func newCoalescer(emit func(data []byte), visibleEvery, hiddenEvery time.Duration) *coalescer {
	return &coalescer{emit: emit, visibleEvery: visibleEvery, hiddenEvery: hiddenEvery, visible: true}
}

// Write ingests one PTY read. A visible write that lands on an idle terminal
// is emitted synchronously — there is no burst to collapse, so the window
// would be latency bought for nothing. Otherwise output buffers until the
// flush timer fires — at the current visibility's cadence — visibility flips
// to visible, or the buffer overflows. A hidden terminal never takes the
// leading edge: it does not paint, so it has no latency to protect. data may
// be reused by the caller after Write returns.
func (c *coalescer) Write(data []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || len(data) == 0 {
		return
	}
	leading := c.visible && len(c.pending) == 0 && time.Since(c.lastEmit) >= c.visibleEvery
	c.pending = append(c.pending, data...)
	if leading || len(c.pending) >= maxPendingBytes {
		c.flushLocked()
		return
	}
	if c.timer == nil {
		c.timer = time.AfterFunc(c.intervalLocked(), c.onTimer)
	}
}

// intervalLocked picks the flush cadence for the current visibility. Caller
// holds mu.
func (c *coalescer) intervalLocked() time.Duration {
	if c.visible {
		return c.visibleEvery
	}
	return c.hiddenEvery
}

// SetVisible flips the session's visibility. Turning visible flushes any
// pending output immediately so the terminal catches up before it is shown.
func (c *coalescer) SetVisible(visible bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.visible = visible
	if visible {
		c.flushLocked()
	}
}

// Close flushes any remaining output and seals the coalescer; later Writes are
// dropped. Callers emit the exit event after Close so final bytes arrive first.
func (c *coalescer) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	c.closed = true
	c.flushLocked()
}

// flushLocked emits pending output and disarms the timer. Caller holds mu.
func (c *coalescer) flushLocked() {
	if c.timer != nil {
		c.timer.Stop()
		c.timer = nil
	}
	if len(c.pending) == 0 {
		return
	}
	c.emit(c.pending)
	c.pending = nil
	c.lastEmit = time.Now()
}

// onTimer flushes hidden output when the batch window elapses. A callback that
// lost the race with Stop finds pending already drained and does nothing.
func (c *coalescer) onTimer() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.timer = nil
	c.flushLocked()
}
