package terminal

import (
	"encoding/base64"
	"math"
	"strings"
	"time"
)

// readBufSize is the chunk size read from a session's PTY per iteration.
const readBufSize = 32 * 1024

// stream copies PTY output to the frontend until the PTY is closed, then reaps
// the process, drops the session and emits its exit event. Output goes through
// the session's coalescer, which batches it on a short cadence while the
// terminal is visible and a long one while it is hidden, and then through its
// outbox, which delivers it off this goroutine.
func (s *Service) stream(id string, sess *session) {
	p := sess.pty
	buf := make([]byte, readBufSize)
	for {
		n, err := p.Read(buf)
		if n > 0 {
			s.noteOutput(id, sess, buf[:n])
			sess.replay.append(buf[:n])
			sess.out.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}
	code, _ := p.Wait()
	// Release the PTY handle: on a natural child exit nobody else closes it
	// (Close only reaps sessions still in the map), and after a user-driven
	// Close this is the second one — a no-op each seam has to make safe, since
	// the handles are already gone (see windowsPTY, where they are reissued).
	_ = p.Close()
	// Flush any batched output and wait for it to be delivered before the exit
	// event, so the frontend always sees the final bytes ahead of the exit
	// banner. Delivery now ends at the transport's writer rather than at the
	// socket, so the wait runs one step further: the outbox for this session's
	// own frames, then the connection's queue for the wire.
	sess.out.Close()
	sess.outbox.close()
	if s.ws != nil {
		s.ws.flush()
	}

	s.mu.Lock()
	reaped := false
	if current, ok := s.sessions[id]; ok && current.pty == p {
		delete(s.sessions, id)
		close(current.done)
		reaped = true
	}
	// Inside the same guard the map delete is: a reap that lost the race to a
	// respawn under this id must not drop the kind the live session was
	// registered with.
	if reaped {
		s.spawns.Delete(id)
	}
	s.mu.Unlock()

	// Only the goroutine that actually evicted its own PTY owes an exit event:
	// a reap that lost the race to a respawn under the same id would otherwise
	// tell a live session it exited, and the frontend writes "[process exited]"
	// into that terminal. Emitted outside s.mu for the reason Start gives —
	// Emit blocks on a stalled /events client, and holding s.mu across it would
	// freeze every session's I/O.
	if reaped {
		s.hub.Emit(exitEventPrefix+id, exitPayload(code))
	}
}

// Write forwards keyboard input from the frontend to a session's PTY.
func (s *Service) Write(id, data string) error {
	return s.writeBytes(id, []byte(data))
}

// writeBytes delivers input bytes to a session's PTY; unknown sessions are a
// no-op. It is the shared sink for the RPC Write and the WebSocket
// transport's input frames.
//
// It loops until every byte is written. A single call is not enough:
// ConPTY's input pipe (pty_windows.go) is a plain Windows anonymous pipe, and
// WriteFile on one can legitimately return fewer bytes than given rather than
// blocking for the rest — the caller is documented to retry. A large paste
// landing exactly on that boundary lost its tail silently, closing bracket
// and all, so bracketed paste never closed and the Enter that followed
// landed inside it instead of submitting it. Unix's *os.File already loops
// internally, so this is a no-op extra check there.
func (s *Service) writeBytes(id string, data []byte) error {
	p := s.ptyOf(id)
	if p == nil {
		return nil
	}
	for len(data) > 0 {
		n, err := p.Write(data)
		if err != nil {
			return err
		}
		data = data[n:]
	}
	return nil
}

// SetVisible tells the session's output coalescer whether its terminal is on
// screen. Hidden sessions batch output (~250ms per event); flipping to visible
// flushes pending output immediately. Unknown sessions are a no-op.
func (s *Service) SetVisible(id string, visible bool) error {
	s.mu.Lock()
	sess, ok := s.sessions[id]
	s.mu.Unlock()
	if !ok {
		return nil
	}
	sess.out.SetVisible(visible)
	return nil
}

// Replay returns the capped tail of a session's PTY output, base64-encoded, so a
// reconnecting frontend can reseed its scrollback after a page reload discarded
// the page-side buffer. Empty for an unknown session — a brand-new one has no
// history yet. Base64 for the same reason the data event is: raw PTY bytes may
// split a multi-byte UTF-8 sequence across the JSON envelope.
func (s *Service) Replay(id string) (string, error) {
	s.mu.Lock()
	sess, ok := s.sessions[id]
	s.mu.Unlock()
	if !ok {
		return "", nil
	}
	return base64.StdEncoding.EncodeToString(sess.replay.snapshot()), nil
}

// Resize updates a session's PTY window size. The frontend only calls this for
// the visible terminal; a hidden terminal is resized on the next time it is
// shown.
func (s *Service) Resize(id string, cols, rows int) error {
	// Recorded even when the session it names is gone: it is the window's own
	// terminal being measured, and the next session spawned without one starts
	// at that size (see sizeFor).
	if cols > 0 && rows > 0 {
		s.mu.Lock()
		s.lastCols, s.lastRows = cols, rows
		s.mu.Unlock()
	}
	p := s.ptyOf(id)
	if p == nil {
		return nil
	}
	return p.Resize(cols, rows)
}

// noteOutput reads one chunk of a session's output for the three things lich
// has to know about it from outside: whether the worktree setup script is still
// the program on the other end of this PTY (see setupDone), whether that
// program has ever stopped drawing (see Ready), and whether the agent is
// working right now (see handsOn).
func (s *Service) noteOutput(id string, sess *session, chunk []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// The quiet is recorded as it passes, never sampled when somebody finally
	// asks. A session settles seconds after it starts and is asked hours later,
	// by then mid-turn: sampling there reads a spinner redrawing every few
	// frames and calls a session that has been at its prompt all day unready.
	//
	// A setup script pauses too — between one package and the next — and that
	// quiet belongs to a program the provider has not replaced yet, so it is
	// not the provider's and does not count.
	if !sess.settingUp && !sess.lastOut.IsZero() && time.Since(sess.lastOut) >= readySettle {
		sess.ready = true
	}
	sess.lastOut = time.Now()
	if sess.settingUp && sess.setupEnded(chunk) {
		sess.settingUp = false
		// The provider starts drawing from here, so the quiet this waits for is
		// measured from the exec, not from the setup script's last line. The
		// clock goes with the flag: the exec itself takes a second or two — the
		// image is replaced, a runtime starts, a splash is composed — and that
		// silence is nobody's quiet. Measured from the script's last line it
		// would clear the settle on the provider's very first byte, mid-splash,
		// which is the write that lands on screen as literal paste markers.
		sess.ready = false
		sess.lastOut = time.Time{}
	}
	// Output is the one beat source that has to be qualified, because a
	// terminal draws for reasons that are nobody's work: a `tail -f`, a dev
	// server logging requests, a TUI repainting its own spinner. An open turn
	// is what separates the agent working from a program talking to itself —
	// so nothing here counts unless the session's own last report said `busy`,
	// and even then only once every handsOnOutputBeat.
	if s.turns.busy(id) {
		s.beatHandsOn(id, handsOnOutputBeat)
	}
}

// setupEnded reports whether chunk completes the wrapper's end marker, matching
// across the seam between two reads: the wrapper prints the marker in one write,
// but a PTY read can cut it anywhere, and neither half matches on its own — a
// session that missed it waits out every relay it is ever sent.
//
// Called under s.mu.
func (sess *session) setupEnded(chunk []byte) bool {
	joined := sess.setupTail + string(chunk)
	if strings.Contains(joined, setupDone) {
		sess.setupTail = ""
		return true
	}
	// One byte short of the marker is the most that can still be its beginning.
	if keep := len(setupDone) - 1; len(joined) > keep {
		joined = joined[len(joined)-keep:]
	}
	sess.setupTail = joined
	return false
}

// Ready reports whether a session can be given work — whether what reads this
// PTY is the provider rather than the project's setup script, and whether the
// prompt is free rather than half-filled by the person sitting at it. False for
// a session that is not running at all.
//
// Live is not this question. A session whose checkout is still installing its
// dependencies has a PTY, appears in the roster, and accepts writes that go
// straight into `pnpm install`'s stdin and are discarded before the provider
// ever starts. It looked delivered to everyone involved: the sender waited out
// its ticket on an agent that was never asked anything.
func (s *Service) Ready(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[id]
	if !ok || sess.settingUp {
		return false
	}
	// Unlike the rest of this, being ready is not a state a session reaches and
	// keeps: the user starts typing and the prompt is theirs again until they
	// send it (see draft.go).
	if sess.drafting(time.Now()) {
		return false
	}
	if sess.ready {
		return true
	}
	// A TUI that has stopped drawing is one that has taken the terminal and is
	// waiting on input. Before that, a message is written into a program still
	// setting the tty up, which discards what it finds there — the bracketed
	// paste then lands on screen as literal text, ahead of a prompt that never
	// received it.
	//
	// Quiet right now answers it too, for the session that has not produced a
	// byte since its last one; a quiet that passed earlier was recorded when it
	// happened (see noteOutput). Either way the session stays ready from then
	// on. A busy agent draws continuously, and a target mid-turn has always
	// been written to — its provider queues the input and answers a turn later.
	if !sess.lastOut.IsZero() && time.Since(sess.lastOut) >= readySettle {
		sess.ready = true
		return true
	}
	return false
}

// QuietFor reports how long a session's PTY has produced nothing, which is the
// only way from outside to tell that the program on the other end has finished
// taking in what was typed at it. A session that has never written a byte — and
// one nobody knows — is as quiet as a session gets.
//
// Ready answers a coarser version of the same question once, and latches; this
// one is asked again in the middle of a delivery, where the quiet that matters
// is the one happening right now (internal/relay, awaitSettled).
func (s *Service) QuietFor(id string) time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[id]
	if !ok || sess.lastOut.IsZero() {
		return quietForever
	}
	return time.Since(sess.lastOut)
}

// quietForever is the answer for a session with no output to time from. Large
// enough that every caller reads it as "settled", and a duration rather than a
// second return value because there is nothing here a caller would do
// differently.
const quietForever = time.Duration(math.MaxInt64)
