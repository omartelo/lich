package terminal

import "time"

// What the user has typed at a prompt and not sent yet, and why lich has to
// know: a relayed message is pasted straight into that prompt and an Enter is
// sent behind it (internal/relay, deliver), so one landing mid-sentence sends
// lich's text and the user's half-written one as a single submission. The
// user's half is gone — it was never in a scrollback, and no provider offers a
// way to get it back.
//
// Only the frontend's keystrokes reach here; what the relay itself types goes
// in through Write. The two paths were already separate, which is what makes
// this answerable at all — and answerable the same way for every provider,
// because it is asked of the PTY rather than of the TUI drawing on it.

const (
	// draftIdle is how long unsent input keeps a prompt to itself. It bounds a
	// heuristic that cannot be exact: what is on the line is the TUI's business
	// and lich only sees the bytes going in, so an edit lich cannot read as one —
	// Ctrl+W, a click into the middle of the line — would otherwise leave a draft
	// that never clears and a relay that never delivers into that session again,
	// silently. Long enough to cover a pause mid-sentence, short against the
	// budget a held-back delivery has to wait it out (relay's deliveryLimit).
	draftIdle = time.Minute

	esc   = 0x1b
	ctrlC = 0x03
	// maxEscape bounds what is held waiting for the rest of a sequence. The
	// longest one a terminal sends is a mouse report at around a dozen bytes; an
	// ESC that never ends is not a sequence at all, and holding it would swallow
	// everything typed after it.
	maxEscape = 32
)

// noteInput records whether the user is composing something at this session's
// prompt right now, so a relayed message is not pasted into the middle of it
// (see Ready). Unknown sessions are a no-op.
func (s *Service) noteInput(id string, data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[id]
	if !ok {
		return
	}
	if len(sess.escPending) > 0 {
		data = append(sess.escPending, data...)
		sess.escPending = nil
	}
	for i := 0; i < len(data); i++ {
		switch b := data[i]; {
		case b == esc:
			n, done := escapeSpan(data[i:])
			if !done {
				// The rest of it is in the next write. Held rather than counted:
				// its parameters are digits, and digits read as typing.
				if n <= maxEscape {
					sess.escPending = append([]byte(nil), data[i:]...)
				}
				return
			}
			i += n - 1
		case b == '\r' || b == '\n' || b == ctrlC:
			// Sent, or thrown away. Either way the line is the user's no longer.
			sess.draftAt = time.Time{}
		case b >= 0x20:
			sess.draftAt = time.Now()
		}
	}
}

// escapeSpan is how many bytes the escape sequence starting at data[0] takes
// up, and whether it ended inside data at all — a PTY write can split one, and
// what arrives first has to be recognised as a beginning rather than as text.
//
// Sequences are skipped rather than interpreted: arrow keys arrive as input,
// and with mouse tracking on (?1003h) so does every pointer movement over the
// terminal. Counting the printable bytes inside those as typing would leave a
// session whose prompt never looks free again.
func escapeSpan(data []byte) (int, bool) {
	if len(data) < 2 {
		return len(data), false
	}
	if data[1] != '[' && data[1] != 'O' {
		return 2, true // ESC + one key, which is how Alt+<key> arrives.
	}
	for i := 2; i < len(data); i++ {
		// A CSI runs to its final byte; everything before it is parameters.
		if data[i] >= 0x40 && data[i] <= 0x7e {
			return i + 1, true
		}
	}
	return len(data), false
}

// drafting is whether the user has unsent input at this prompt as of now.
// Called under the service's mu.
func (sess *session) drafting(now time.Time) bool {
	return !sess.draftAt.IsZero() && now.Sub(sess.draftAt) < draftIdle
}
