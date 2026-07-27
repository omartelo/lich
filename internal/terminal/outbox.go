package terminal

// outboxDepth is how many flushed frames may wait on a stalled frontend before
// the session's own PTY read blocks. A frame is one coalescer flush — 8ms of
// output while visible, capped at maxPendingBytes — so this bounds both the
// memory one session may hold and how far ahead of the window it may run.
const outboxDepth = 32

// outbox carries one session's coalesced output to the frontend on a goroutine
// of its own.
//
// Delivery blocks: one WebSocket carries every session (transport.send) and
// the /events fallback is one socket too, so a window that stops reading — the
// page busy parsing a resumed session's transcript is enough — backs pressure
// up into whoever is writing. Doing that on the PTY read path stalls the read,
// the PTY buffer fills, and the child stops producing; because the socket is
// shared, that reaches every session, not just the slow one. The queue is what
// decouples them: a stalled window backs up one session's frames and, only
// once the queue is full, that session's PTY. The backpressure that remains is
// honest — it reaches the session that is producing, never its neighbours.
type outbox struct {
	frames chan []byte
	done   chan struct{}
}

// newOutbox starts the delivery goroutine. deliver is called with one frame at
// a time, in order, and owns the slice it is handed.
func newOutbox(deliver func(data []byte), depth int) *outbox {
	o := &outbox{frames: make(chan []byte, depth), done: make(chan struct{})}
	go func() {
		defer close(o.done)
		for frame := range o.frames {
			deliver(frame)
		}
	}()
	return o
}

// push queues one frame for delivery, blocking only while this session's queue
// is full. The frame is copied: the coalescer reuses the buffer it passes.
func (o *outbox) push(data []byte) {
	o.frames <- append([]byte(nil), data...)
}

// close stops the queue and waits for the frames still in it to be delivered,
// so the caller can order a later event (the exit banner) after the last byte.
func (o *outbox) close() {
	close(o.frames)
	<-o.done
}
