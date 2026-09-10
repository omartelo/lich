package terminal

import (
	"sync"
	"testing"
	"time"
)

// fakeClock is a hand-wound clock: the accumulator's whole contract is about
// gaps of minutes, and a test that slept through them would be the slowest in
// the suite and still flaky.
type fakeClock struct {
	mu sync.Mutex
	at time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{at: time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.at
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.at = c.at.Add(d)
}

// drainSeconds is what one drain would write for a session, in whole seconds.
func drainSeconds(h *handsOn, id string) int64 {
	return h.drain()[id]
}

// TestFirstBeatCountsNothing proves a session's first beat opens the chain and
// credits nothing: there is no earlier beat to measure against, and inventing a
// start would bill the session for however long lich had been running.
func TestFirstBeatCountsNothing(t *testing.T) {
	clock := newFakeClock()
	h := &handsOn{now: clock.now}

	h.beat("s1", 0)

	if got := drainSeconds(h, "s1"); got != 0 {
		t.Errorf("first beat counted %ds, want 0", got)
	}
}

// TestGapUnderIdleGapCounts proves the ordinary case: two beats a few minutes
// apart are minutes at the keyboard, and the whole gap is credited.
func TestGapUnderIdleGapCounts(t *testing.T) {
	clock := newFakeClock()
	h := &handsOn{now: clock.now}

	h.beat("s1", 0)
	clock.advance(5 * time.Minute)
	h.beat("s1", 0)

	if got := drainSeconds(h, "s1"); got != 300 {
		t.Errorf("a 5m gap counted %ds, want 300", got)
	}
}

// TestGapAtIdleGapCounts pins the boundary itself: exactly handsOnIdleGap is
// still work. The literal is written out rather than derived from the constant,
// so moving the constant fails this test instead of silently moving the rule
// with it.
func TestGapAtIdleGapCounts(t *testing.T) {
	clock := newFakeClock()
	h := &handsOn{now: clock.now}

	h.beat("s1", 0)
	clock.advance(15 * time.Minute)
	h.beat("s1", 0)

	if got := drainSeconds(h, "s1"); got != 900 {
		t.Errorf("a gap of exactly the idle gap counted %ds, want 900", got)
	}
}

// TestGapOverIdleGapCountsNothing is the rule the whole readout rests on: a
// silence longer than the idle gap is time away, and it contributes zero rather
// than a capped amount — a session left open overnight must not bill the night.
func TestGapOverIdleGapCountsNothing(t *testing.T) {
	clock := newFakeClock()
	h := &handsOn{now: clock.now}

	h.beat("s1", 0)
	clock.advance(15*time.Minute + time.Second)
	h.beat("s1", 0)

	if got := drainSeconds(h, "s1"); got != 0 {
		t.Errorf("a gap past the idle gap counted %ds, want 0", got)
	}
}

// TestSilenceStillRearmsTheChain proves a long silence costs only itself: the
// beat that ends it is the start of the next stretch, and the work after it is
// counted in full.
func TestSilenceStillRearmsTheChain(t *testing.T) {
	clock := newFakeClock()
	h := &handsOn{now: clock.now}

	h.beat("s1", 0)
	clock.advance(2 * time.Hour)
	h.beat("s1", 0)
	clock.advance(3 * time.Minute)
	h.beat("s1", 0)

	if got := drainSeconds(h, "s1"); got != 180 {
		t.Errorf("work after a long silence counted %ds, want 180", got)
	}
}

// TestThrottleDropsCloseBeats proves the gate the output path depends on: beats
// closer together than minGap are dropped whole — neither counted nor recorded
// as the previous beat — so a stream of them cannot chain its way to a total.
func TestThrottleDropsCloseBeats(t *testing.T) {
	clock := newFakeClock()
	h := &handsOn{now: clock.now}

	h.beat("s1", handsOnOutputBeat)
	for range 100 {
		clock.advance(time.Second)
		h.beat("s1", handsOnOutputBeat)
	}

	// 100 seconds of steady output, throttled to a beat every 30: three gaps of
	// 30 seconds land, and the last 10 seconds are still waiting for their beat.
	if got := drainSeconds(h, "s1"); got != 90 {
		t.Errorf("100s of throttled output counted %ds, want 90", got)
	}
}

// TestThrottleAdmitsBeatAtTheGap pins the throttle's own boundary: a beat
// exactly minGap after the last one is admitted, not dropped.
func TestThrottleAdmitsBeatAtTheGap(t *testing.T) {
	clock := newFakeClock()
	h := &handsOn{now: clock.now}

	h.beat("s1", handsOnOutputBeat)
	clock.advance(30 * time.Second)
	h.beat("s1", handsOnOutputBeat)

	if got := drainSeconds(h, "s1"); got != 30 {
		t.Errorf("a beat at exactly the throttle counted %ds, want 30", got)
	}
}

// TestBackwardsClockCountsNothing proves a clock that jumps back — an NTP step,
// a laptop resuming — neither credits negative time nor corrupts the chain.
func TestBackwardsClockCountsNothing(t *testing.T) {
	clock := newFakeClock()
	h := &handsOn{now: clock.now}

	h.beat("s1", 0)
	clock.advance(-time.Hour)
	h.beat("s1", 0)
	clock.advance(time.Hour + 2*time.Minute)
	h.beat("s1", 0)

	if got := drainSeconds(h, "s1"); got != 120 {
		t.Errorf("a backwards clock left %ds counted, want 120", got)
	}
}

// TestDrainKeepsTheRemainder proves the sub-second leftover survives a drain.
// At two flushes a minute, throwing it away would lose most of a minute an hour
// and the readout would drift quietly low.
func TestDrainKeepsTheRemainder(t *testing.T) {
	clock := newFakeClock()
	h := &handsOn{now: clock.now}

	h.beat("s1", 0)
	clock.advance(1500 * time.Millisecond)
	h.beat("s1", 0)
	if got := drainSeconds(h, "s1"); got != 1 {
		t.Fatalf("first drain wrote %ds, want 1", got)
	}

	clock.advance(1500 * time.Millisecond)
	h.beat("s1", 0)
	if got := drainSeconds(h, "s1"); got != 2 {
		t.Errorf("second drain wrote %ds, want 2 — the remainder was dropped", got)
	}
}

// TestDrainSkipsQuietSessions proves an app full of idle cards drains to
// nothing, so the debounced write touches no row it has nothing to say about.
func TestDrainSkipsQuietSessions(t *testing.T) {
	clock := newFakeClock()
	h := &handsOn{now: clock.now}

	h.beat("quiet", 0)
	h.beat("worked", 0)
	clock.advance(time.Minute)
	h.beat("worked", 0)

	drained := h.drain()
	if _, ok := drained["quiet"]; ok {
		t.Error("a session that counted nothing was drained")
	}
	if drained["worked"] != 60 {
		t.Errorf("worked drained %ds, want 60", drained["worked"])
	}
}

// TestSessionsAreCountedApart proves the accumulator keys everything by session:
// one card's beats never advance another's chain.
func TestSessionsAreCountedApart(t *testing.T) {
	clock := newFakeClock()
	h := &handsOn{now: clock.now}

	h.beat("s1", 0)
	clock.advance(time.Minute)
	h.beat("s2", 0)
	clock.advance(time.Minute)
	h.beat("s1", 0)

	drained := h.drain()
	if drained["s1"] != 120 {
		t.Errorf("s1 counted %ds, want 120", drained["s1"])
	}
	if _, ok := drained["s2"]; ok {
		t.Errorf("s2 counted %ds off its own first beat, want nothing", drained["s2"])
	}
}

// TestFlushIsDueOncePerWindow proves exactly one beat per window is told to
// write: the debounce is armed by whoever crosses the line, so two beats a
// millisecond apart cannot both spawn a writer.
func TestFlushIsDueOncePerWindow(t *testing.T) {
	clock := newFakeClock()
	h := &handsOn{now: clock.now, lastFlush: clock.now()}

	if h.beat("s1", 0) {
		t.Error("a beat inside the window asked for a write")
	}
	clock.advance(handsOnFlush)
	if !h.beat("s1", 0) {
		t.Fatal("the beat that crossed the window did not ask for a write")
	}
	if h.beat("s2", 0) {
		t.Error("a second beat in the same window asked for a write too")
	}
}

// TestForgetBreaksTheChain proves a card closed and reopened under the same id
// does not count the gap it was gone for.
func TestForgetBreaksTheChain(t *testing.T) {
	clock := newFakeClock()
	h := &handsOn{now: clock.now}

	h.beat("s1", 0)
	h.forget("s1")
	clock.advance(time.Minute)
	h.beat("s1", 0)

	if got := drainSeconds(h, "s1"); got != 0 {
		t.Errorf("the gap across a forget counted %ds, want 0", got)
	}
}

// TestZeroValueAccumulatorWorks proves the zero value is usable, which is the
// state every bare Service the tests build is in — a readout must never be the
// reason a keystroke panics.
func TestZeroValueAccumulatorWorks(t *testing.T) {
	var h handsOn

	h.beat("s1", 0)
	h.forget("s1")
	if got := h.drain(); len(got) != 0 {
		t.Errorf("a zero-value accumulator drained %v, want nothing", got)
	}
}
