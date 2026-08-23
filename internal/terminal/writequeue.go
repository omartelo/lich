package terminal

import (
	"context"
	"sync"

	"github.com/coder/websocket"
)

// writeQueueDepth is how many frames may wait on the connection's writer before
// the session pushing one waits with it. A frame is one coalescer flush, capped
// at maxPendingBytes, so this bounds what a stalled socket may hold in memory —
// twice the depth one session's own outbox holds, because this queue is the one
// every session shares.
const writeQueueDepth = 64

// writeQueue owns one client connection's socket: every session's output frame
// crosses it on a single goroutine.
//
// A WebSocket admits one writer at a time, so something has to serialize the
// sessions. Doing it on a mutex the senders take is what this replaces:
// conn.Write is bounded only by wsWriteTimeout, and every session's outbox
// goroutine (outbox.go) went through that mutex, so one stalled write froze
// every session's delivery behind it — the per-session decoupling the outbox
// exists to provide, undone one layer down. Here the socket belongs to the
// writer alone: a session hands over a frame and returns, and the stall costs
// the connection, never its neighbours.
//
// Frames leave in the order they arrived, so a session's output reaches the
// client exactly as its outbox produced it. None is discarded: a frame the
// socket refuses — and everything queued behind it — goes to the fallback the
// transport hands over, which is the same /events bridge send already falls
// back to when there is no client at all. A push that arrives once the queue is
// closed waits for that drain before it reports failure, so the caller's own
// fallback lands behind those frames rather than in front of them.
type writeQueue struct {
	mu   sync.Mutex
	wake *sync.Cond
	// frames holds encoded frames; the transport allocates each one, so the
	// queue never copies. inflight is the frame the writer has taken but not
	// finished with, which flush and a closed push both have to wait out.
	frames   [][]byte
	inflight int
	closed   bool
}

func newWriteQueue() *writeQueue {
	q := &writeQueue{}
	q.wake = sync.NewCond(&q.mu)
	return q
}

// push queues one encoded frame, blocking only while the queue is full — never
// while the transport's own lock is held. It reports false once the queue is
// closed, and only after everything already queued has been drained, so the
// caller may fall back for this frame without overtaking the ones before it.
func (q *writeQueue) push(frame []byte) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	for len(q.frames) >= writeQueueDepth && !q.closed {
		q.wake.Wait()
	}
	if q.closed {
		q.waitDrained()
		return false
	}
	q.frames = append(q.frames, frame)
	q.wake.Broadcast()
	return true
}

// pop hands the writer the next frame, waiting for one. ok is false once the
// queue is closed and drained, which is the writer's signal to stop.
func (q *writeQueue) pop() (frame []byte, ok bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for len(q.frames) == 0 && !q.closed {
		q.wake.Wait()
	}
	if len(q.frames) == 0 {
		return nil, false
	}
	frame, q.frames = q.frames[0], q.frames[1:]
	q.inflight++
	return frame, true
}

// settle marks the frame the writer last popped as dealt with, whether the
// socket carried it or the fallback did.
func (q *writeQueue) settle() {
	q.mu.Lock()
	q.inflight--
	q.wake.Broadcast()
	q.mu.Unlock()
}

// flush waits until every frame pushed so far has left the queue and the writer
// has finished with it. A session emits its exit event after this, so the banner
// still cannot overtake the session's own last bytes.
func (q *writeQueue) flush() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.waitDrained()
}

// waitDrained blocks until nothing is queued or in the writer's hands. Callers
// hold q.mu. A closed queue still drains — the writer hands the rest to the
// fallback — so this is bounded either way.
func (q *writeQueue) waitDrained() {
	for len(q.frames) > 0 || q.inflight > 0 {
		q.wake.Wait()
	}
}

// close retires the queue. The writer drains what is left to the fallback and
// then stops; close itself does not wait, so a page reload never blocks the
// incoming client on the outgoing one.
func (q *writeQueue) close() {
	q.mu.Lock()
	q.closed = true
	q.wake.Broadcast()
	q.mu.Unlock()
}

// run is the writer goroutine: it puts frames on conn until one fails, calls
// failed once when that happens, and hands that frame and every one after it to
// fallback so nothing an outbox delivered is lost. A nil fallback discards,
// which is the transport's own answer when no bridge has been wired yet.
func (q *writeQueue) run(conn *websocket.Conn, failed func(), fallback func(id string, data []byte)) {
	live := true
	for {
		frame, ok := q.pop()
		if !ok {
			return
		}
		if live && q.write(conn, frame) {
			q.settle()
			continue
		}
		if live {
			live = false
			failed()
		}
		if fallback != nil {
			// The frame is its own record of which session it belongs to, so the
			// queue carries one encoded slice rather than a slice and its parts.
			if id, data, err := decodeFrame(frame); err == nil {
				fallback(id, data)
			}
		}
		q.settle()
	}
}

// write puts one frame on the socket under the send timeout, reporting whether
// the client took it.
func (q *writeQueue) write(conn *websocket.Conn, frame []byte) bool {
	ctx, cancel := context.WithTimeout(context.Background(), wsWriteTimeout)
	defer cancel()
	return conn.Write(ctx, websocket.MessageBinary, frame) == nil
}
