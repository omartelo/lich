package quota

import "time"

// Pace is what a weekly window's percentage cannot say on its own: a window 40%
// spent on day two and one 40% spent on day six draw exactly the same bar, and
// only one of them is going to run out. Ahead marks the first — spend running
// far enough past the share of the window that has already elapsed that the
// rate, not the number, is the thing worth seeing.
const (
	// aheadPoints is how far past the elapsed share spend has to run before the
	// window is marked. Anything tighter marks an ordinary heavy afternoon in an
	// otherwise light week, and a marker that is on most of the time is a marker
	// nobody reads.
	aheadPoints = 15
	// aheadGrace is how long after a reset no window is marked. Just after one,
	// the elapsed share is near zero and expected spend with it, so almost any
	// use at all reads as far ahead — a false positive, not a warning. An
	// `elapsed == 0` guard does not cover this: a reading taken an hour into a
	// fresh window has a small elapsed, not a zero one.
	aheadGrace = 24 * time.Hour
)

// markAhead marks w when its spend runs ahead of the window's own clock. Pure:
// `now` is a parameter so the verdict is testable and no reading depends on a
// clock read somewhere else.
//
// Only the weekly window is paced — the account-wide one and the model-scoped
// caps alike, which are weekly too. The five-hour window is left out on purpose:
// it turns over too fast for a rate to mean anything, since the first spend in a
// fresh window is "ahead" almost by definition, and the package's five-minute
// cache is a large fraction of five hours while it is nothing against a week.
//
// The 15-point threshold and the 24-hour grace are calibration measured by the
// claude-swap project (`src/claude_swap/pace.py`, its issue #125). The numbers
// are what is borrowed from there; the code is lich's own.
func markAhead(w Window, now time.Time) Window {
	if w.Seconds != weeklyWindow {
		return w
	}
	start, ok := windowStart(w, now)
	if !ok {
		return w
	}
	elapsed := now.Sub(start)
	if elapsed < aheadGrace {
		return w
	}
	expected := elapsed.Seconds() / float64(w.Seconds) * 100
	w.Ahead = float64(w.Percent)-expected >= aheadPoints
	return w
}

// windowStart derives when the current cycle began. ResetsAt is the only
// timestamp either provider reports and it is the *next* reset, never the start,
// so the start is that reset rolled back whole windows until it lands at or
// before now — which stays right however many cycles ahead a reset time sits,
// rather than assuming it is exactly one.
//
// False for a window with no parseable reset or no length: with no cycle to
// place, there is no pace to read, and an unmarked window is the honest answer.
func windowStart(w Window, now time.Time) (time.Time, bool) {
	if w.Seconds <= 0 {
		return time.Time{}, false
	}
	reset, err := time.Parse(time.RFC3339, w.ResetsAt)
	if err != nil {
		return time.Time{}, false
	}
	period := time.Duration(w.Seconds) * time.Second
	cycles := int64(1)
	if until := reset.Sub(now); until > period {
		cycles = int64((until + period - 1) / period)
	}
	return reset.Add(-time.Duration(cycles) * period), true
}

// pace marks every window of a reading. One call site for every provider,
// because the verdict is derived from a Window alone: a provider that starts
// reporting a weekly window is paced without knowing this exists.
func pace(plans []Plan, now time.Time) {
	for _, p := range plans {
		for i, w := range p.Windows {
			p.Windows[i] = markAhead(w, now)
		}
	}
}
