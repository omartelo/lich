package terminal

import "sync"

// replayCapBytes caps a session's server-side replay tail, matching the
// frontend replay buffer (replay-buffer.ts). Per running session, in memory —
// a personal harness runs a handful; a disk-backed store (waveterm's filestore)
// is the upgrade path if session count or size ever makes that cost matter.
const replayCapBytes = 2 << 20

// replayBuffer keeps a capped tail of a session's raw PTY output so a
// reconnecting frontend can reseed its scrollback — most importantly after a
// full page reload, which discards the page-side buffer while the PTY (and this
// tail) live on in the backend. It mirrors the frontend replay buffer: a chunk
// list dropped from the front on overflow, so after a drop the head may be a
// partial ANSI sequence — the same accepted artifact (TUIs repaint, shells
// scroll it away). Safe for concurrent append (the stream goroutine) and
// snapshot (an RPC call).
type replayBuffer struct {
	mu       sync.Mutex
	chunks   [][]byte
	total    int
	capBytes int
}

func newReplayBuffer(capBytes int) *replayBuffer {
	return &replayBuffer{capBytes: capBytes}
}

// append copies data (the caller reuses its read buffer) and enqueues it,
// dropping the oldest chunks while the total exceeds the cap. One chunk is
// always kept, so a single write larger than the cap still replays.
func (b *replayBuffer) append(data []byte) {
	if len(data) == 0 {
		return
	}
	chunk := make([]byte, len(data))
	copy(chunk, data)

	b.mu.Lock()
	defer b.mu.Unlock()
	b.chunks = append(b.chunks, chunk)
	b.total += len(chunk)
	for b.total > b.capBytes && len(b.chunks) > 1 {
		b.total -= len(b.chunks[0])
		b.chunks = b.chunks[1:]
	}
}

// snapshot returns the queued tail as one contiguous slice in arrival order.
func (b *replayBuffer) snapshot() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]byte, 0, b.total)
	for _, c := range b.chunks {
		out = append(out, c...)
	}
	return out
}
