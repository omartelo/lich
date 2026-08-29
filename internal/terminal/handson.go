package terminal

import (
	"sync"
	"time"
)

// How the hands-on clock is read. Three constants, and each of them is the
// whole design: there are no sessions to open and close and no timer running
// between them, only beats and the gaps between consecutive ones.
const (
	// handsOnIdleGap is the longest silence that still counts as working time.
	// A gap under it is time the user spent reading, thinking or waiting on the
	// agent, which is time on this session; a gap over it is lunch, another
	// window or another day, and it contributes nothing at all — which is what
	// makes a session left open overnight report the hour it was worked rather
	// than the fourteen it was on screen.
	handsOnIdleGap = 15 * time.Minute
	// handsOnOutputBeat is how rarely a session's own output may beat. It is
	// load-bearing, not a performance knob: PTY output arrives in bursts of a
	// few bytes, and an unthrottled beat per burst would turn any stream —
	// `tail -f`, a dev server, a TUI repainting its spinner — into an unbroken
	// chain of tiny gaps that never reaches handsOnIdleGap, and so into hours
	// of "work" nobody did.
	handsOnOutputBeat = 30 * time.Second
	// handsOnFlush is how long counted time may sit in memory before a beat
	// carries it to the store. Losing a beat to a crash costs at most this
	// much per session, which is the trade the readout is worth.
	handsOnFlush = 30 * time.Second
)

// handsOnEntry is one session's accounting: when it last beat, and what has
// been counted but not yet written.
type handsOnEntry struct {
	last    time.Time
	pending time.Duration
}

// handsOn measures how long each session has actually been worked on, without
// ever deciding when a stretch of work starts or ends.
//
// The whole method is beat-and-gap: something happens in a session, and the
// time since the previous thing that happened in it is added to the total —
// unless that gap is longer than handsOnIdleGap, in which case it is a silence
// and adds nothing. Nothing is scheduled, nothing has to be closed, and a lich
// that dies mid-turn leaves no session open behind it: the next beat after a
// restart simply finds no previous one and starts the chain again.
//
// It holds no absolute total. What it keeps is the arrears — whole seconds not
// yet handed to the store, which owns the figure the readout comes from (see
// store.AddHandsOn). That is what lets a beat write with `seconds + ?` and
// makes a lost flush cost only what it was carrying.
//
// The zero value is ready to use, like turnLog beside it: a bare Service is a
// state the tests build, and one that panicked on the first keystroke would be
// a trap set by a readout.
type handsOn struct {
	mu sync.Mutex
	// now is the clock, and nil is the wall clock. The tests set it so a
	// fifteen-minute gap is a line of code rather than a sleep.
	now       func() time.Time
	entries   map[string]handsOnEntry
	lastFlush time.Time
}

// clock reads the injected time source, falling back to the wall clock.
func (h *handsOn) clock() time.Time {
	if h.now == nil {
		return time.Now()
	}
	return h.now()
}

// beat records one moment of activity in session id and reports whether the
// arrears are due a write.
//
// minGap is the throttle: a beat closer than that to the previous one is
// dropped whole, which is how the output path (handsOnOutputBeat) keeps a
// streaming program from beating its way to a number nobody worked. Pass 0 for
// a beat that is already rare — a state report, a keystroke — and the only
// thing dropped is a clock that went backwards.
//
// The first beat of a session counts nothing: there is no previous one to
// measure a gap against, and inventing a start would credit the session with
// however long lich had been running.
func (h *handsOn) beat(id string, minGap time.Duration) (flush bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	now := h.clock()
	entry := h.entries[id]
	if !entry.last.IsZero() {
		gap := now.Sub(entry.last)
		if gap < minGap {
			return false
		}
		if gap > 0 && gap <= handsOnIdleGap {
			entry.pending += gap
		}
	}
	entry.last = now
	if h.entries == nil {
		h.entries = make(map[string]handsOnEntry)
	}
	h.entries[id] = entry
	return h.due(now)
}

// due reports whether handsOnFlush has passed since the last write, and arms
// the next window in the same breath — so of every beat that crosses the line,
// exactly one caller is told to write. Callers hold h.mu.
func (h *handsOn) due(now time.Time) bool {
	if now.Sub(h.lastFlush) < handsOnFlush {
		return false
	}
	h.lastFlush = now
	return true
}

// drain hands back the whole seconds each session has run up since the last
// drain, and keeps the remainder. Keeping it is what stops the readout drifting
// low: at two flushes a minute, rounding the leftover away each time would lose
// most of a minute every hour.
//
// Sessions with nothing to write are left out, so an app full of idle cards
// drains to nothing and writes nothing.
func (h *handsOn) drain() map[string]int64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out map[string]int64
	for id, entry := range h.entries {
		seconds := int64(entry.pending / time.Second)
		if seconds == 0 {
			continue
		}
		entry.pending -= time.Duration(seconds) * time.Second
		h.entries[id] = entry
		if out == nil {
			out = make(map[string]int64)
		}
		out[id] = seconds
	}
	return out
}

// forget drops what is remembered about a session, so the gap across a card's
// close — or across a PTY respawned under the same id — is never counted into
// whatever comes next. Callers drain first: the sub-second remainder this
// throws away is the whole cost of not doing so.
func (h *handsOn) forget(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.entries, id)
}
