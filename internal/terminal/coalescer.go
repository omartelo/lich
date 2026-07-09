package terminal

import (
	"sync"
	"time"
)

const (
	// hiddenFlushInterval is how often a hidden session's buffered output is
	// flushed to the frontend. Hidden terminals don't paint, so latency is
	// irrelevant; batching cuts the event-bridge overhead (IPC + base64 +
	// WASM parse per event) to a few events per second.
	hiddenFlushInterval = 250 * time.Millisecond
	// maxPendingBytes forces an early flush while hidden, bounding both the
	// buffer's memory and the size of a single frontend event.
	maxPendingBytes = 256 * 1024
)

// coalescer batches PTY output for a hidden session and passes it through
// unchanged for a visible one. emit is called with the mutex held, so emissions
// are strictly ordered; emit must not retain the slice.
type coalescer struct {
	mu      sync.Mutex
	emit    func(data []byte)
	every   time.Duration
	visible bool
	pending []byte
	// timer is armed (one-shot) only while hidden data is pending, so an idle
	// hidden session costs nothing.
	timer  *time.Timer
	closed bool
}

// newCoalescer returns a coalescer that starts visible — sessions spawn lazily
// on first view, so a session is always on screen when created.
func newCoalescer(emit func(data []byte), every time.Duration) *coalescer {
	return &coalescer{emit: emit, every: every, visible: true}
}

// Write ingests one PTY read. Visible sessions emit immediately; hidden ones
// buffer until the flush timer fires, visibility flips, or the buffer overflows.
// data may be reused by the caller after Write returns.
func (c *coalescer) Write(data []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || len(data) == 0 {
		return
	}
	if c.visible {
		c.emit(data)
		return
	}
	c.pending = append(c.pending, data...)
	if len(c.pending) >= maxPendingBytes {
		c.flushLocked()
		return
	}
	if c.timer == nil {
		c.timer = time.AfterFunc(c.every, c.onTimer)
	}
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
}

// onTimer flushes hidden output when the batch window elapses. A callback that
// lost the race with Stop finds pending already drained and does nothing.
func (c *coalescer) onTimer() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.timer = nil
	c.flushLocked()
}
