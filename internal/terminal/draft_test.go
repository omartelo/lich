package terminal

import (
	"testing"
	"time"

	"github.com/omartelo/lich/internal/events"
)

// settled is a session that has finished drawing itself: ready for work, unless
// the person at it has the prompt.
func settled(t *testing.T) (*Service, *session) {
	t.Helper()
	svc := New(nil, nil, events.New())
	sess := &session{lastOut: time.Now().Add(-time.Second)}
	svc.mu.Lock()
	svc.sessions["s1"] = sess
	svc.mu.Unlock()
	if !svc.Ready("s1") {
		t.Fatal("a settled session was not ready to begin with")
	}
	return svc, sess
}

// The bug this exists for: a relayed message is pasted at the prompt and an
// Enter is sent behind it, so one arriving while the user is mid-sentence sends
// both halves as one submission and the user's half is gone.
func TestTypingTakesThePromptBackFromTheRelay(t *testing.T) {
	svc, _ := settled(t)

	svc.noteInput("s1", []byte("what about the"))
	if svc.Ready("s1") {
		t.Error("a prompt with the user's own half-written line was offered work")
	}

	svc.noteInput("s1", []byte("\r"))
	if !svc.Ready("s1") {
		t.Error("the line was sent and the prompt stayed held")
	}
}

// Ctrl+C throws the line away rather than sending it. Either way it is not the
// user's anymore, and a prompt held by a draft nobody has is a relay that never
// delivers here again.
func TestAnAbandonedLineReleasesThePrompt(t *testing.T) {
	svc, _ := settled(t)

	svc.noteInput("s1", []byte("half a thought"))
	svc.noteInput("s1", []byte{ctrlC})
	if !svc.Ready("s1") {
		t.Error("a line thrown away with Ctrl+C still held the prompt")
	}
}

// With mouse tracking on, every pointer movement over the terminal arrives as
// input, and the digits inside those reports are printable. Read as typing they
// would hold the prompt forever without anybody touching a key.
func TestPointerAndCursorInputIsNotADraft(t *testing.T) {
	svc, _ := settled(t)

	svc.noteInput("s1", []byte("\x1b[<35;42;7M\x1b[<35;43;7M"))
	svc.noteInput("s1", []byte("\x1b[A\x1b[B\x1bOP"))
	if !svc.Ready("s1") {
		t.Error("moving the mouse over a terminal took its prompt away")
	}

	// The same report split across two PTY writes, which is how a long one
	// arrives: neither half may be counted as typing.
	svc.noteInput("s1", []byte("\x1b[<35;10"))
	svc.noteInput("s1", []byte(";5M"))
	if !svc.Ready("s1") {
		t.Error("a mouse report split across two writes was read as typing")
	}
}

// The prompt cannot be held indefinitely: lich sees the bytes going in, never
// the line itself, so an edit it cannot read as one would wedge the relay
// silently. A draft nobody has touched goes stale and gives the prompt back.
func TestAStaleDraftGivesThePromptBack(t *testing.T) {
	svc, sess := settled(t)

	svc.mu.Lock()
	sess.draftAt = time.Now().Add(-59 * time.Second)
	svc.mu.Unlock()
	if svc.Ready("s1") {
		t.Error("a draft under a minute old was already treated as abandoned")
	}

	svc.mu.Lock()
	sess.draftAt = time.Now().Add(-61 * time.Second)
	svc.mu.Unlock()
	if !svc.Ready("s1") {
		t.Error("a draft nobody touched for over a minute still held the prompt")
	}
}

func TestEscapeSpan(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want int
		done bool
	}{
		{"csi arrow", "\x1b[A", 3, true},
		{"csi mouse report", "\x1b[<35;42;7M", 11, true},
		{"csi with a tail after it", "\x1b[Bx", 3, true},
		{"ss3 function key", "\x1bOP", 3, true},
		{"alt and a letter", "\x1bv", 2, true},
		{"truncated csi", "\x1b[<35;10", 8, false},
		{"escape alone", "\x1b", 1, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, done := escapeSpan([]byte(c.in))
			if got != c.want || done != c.done {
				t.Errorf("escapeSpan(%q) = %d, %v; want %d, %v", c.in, got, done, c.want, c.done)
			}
		})
	}
}

// An ESC that never ends is not a sequence: what comes after it is typing, and
// holding it back forever would leave the relay pasting into a line that has a
// paragraph of the user's on it.
func TestAnEscapeThatNeverEndsDoesNotSwallowTyping(t *testing.T) {
	svc, _ := settled(t)

	svc.noteInput("s1", []byte("\x1b[0123456789012345678901234567890123456789"))
	svc.noteInput("s1", []byte("and then a real sentence"))
	if svc.Ready("s1") {
		t.Error("typing after an unterminated escape was discarded")
	}
}

// Reading the interrupt off the keystrokes is the fallback for the providers
// that report nothing when a turn is stopped. What counts is a key the user
// pressed: a lone Ctrl+C or Escape, never a byte carried in by a paste and
// never one inside an escape sequence.
func TestInterruptIsReadFromALoneKey(t *testing.T) {
	tests := []struct {
		name   string
		writes []string
		want   bool
	}{
		{"ctrl+c", []string{"\x03"}, true},
		{"escape", []string{"\x1b"}, true},
		{"escape after a half-typed line", []string{"never mind", "\x1b"}, true},
		{"plain typing", []string{"hello"}, false},
		{"enter", []string{"\r"}, false},
		{"an arrow key", []string{"\x1b[A"}, false},
		{"a function key", []string{"\x1bOP"}, false},
		{"alt and a letter", []string{"\x1bv"}, false},
		{"a mouse report", []string{"\x1b[<35;42;7M"}, false},
		{"a mouse report split across two writes", []string{"\x1b[<35;10", ";5M"}, false},
		{"ctrl+c inside a pasted block", []string{"\x1b[200~kill \x03 the job\x1b[201~"}, false},
		{"escape inside a pasted block", []string{"\x1b[200~press \x1b to stop\x1b[201~"}, false},
		{"a paste split so a write ends on its escape", []string{"\x1b[200~press \x1b", "[A to stop\x1b[201~"}, false},
		{"ctrl+c typed after the paste ended", []string{"\x1b[200~pasted\x1b[201~", "\x03"}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := settled(t)
			got := false
			for _, w := range tc.writes {
				got = svc.noteInput("s1", []byte(w)) || got
			}
			if got != tc.want {
				t.Fatalf("noteInput(%q) read an interrupt = %v, want %v", tc.writes, got, tc.want)
			}
		})
	}
}

// A session lich is not running has no turn to end and no prompt to hold.
func TestInterruptForAnUnknownSessionIsANoOp(t *testing.T) {
	svc, _ := settled(t)
	if svc.noteInput("nobody", []byte{ctrlC}) {
		t.Fatal("a keystroke for a session lich does not run read as an interrupt")
	}
}
