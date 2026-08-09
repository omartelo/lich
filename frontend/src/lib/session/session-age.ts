// How long a session has been in its current status: the readout the sidebar
// draws beside the status glyph, and the one clock every card reads it from.
// Kept free of React so both the formatting and the clock's cadence are covered
// by the node suite.

const SECOND_MS = 1_000
const MINUTE_MS = 60_000
const HOUR_MS = 3_600_000

// formatAge renders an elapsed time for the sidebar, where it sits beside the
// status glyph at 12px and must not compete with the session's label: one unit,
// no padding, no second term. The unit follows the scale — seconds while a turn
// is young enough for them to carry information, then minutes, then hours — and
// each is floored, so the readout never claims time that has not passed. A
// backwards clock jump (an NTP correction) would otherwise read as a negative
// age, so anything below zero counts as none.
export function formatAge(ms: number): string {
  const elapsed = Math.max(0, ms)
  if (elapsed < MINUTE_MS) {
    return `${Math.floor(elapsed / SECOND_MS)}s`
  }
  if (elapsed < HOUR_MS) {
    return `${Math.floor(elapsed / MINUTE_MS)}m`
  }
  return `${Math.floor(elapsed / HOUR_MS)}h`
}

// nextChangeMs is how long until formatAge(ms) returns a different string: the
// rest of the second it is counting in, then of the minute, then of the hour.
// The clock below schedules on this rather than on a fixed interval, for the
// reason the footer's clock does (FooterBar.useNow): a fixed tick either
// redraws a string that did not change or shows one that is already stale.
export function nextChangeMs(ms: number): number {
  const elapsed = Math.max(0, ms)
  if (elapsed < MINUTE_MS) {
    return SECOND_MS - (elapsed % SECOND_MS)
  }
  if (elapsed < HOUR_MS) {
    return MINUTE_MS - (elapsed % MINUTE_MS)
  }
  return HOUR_MS - (elapsed % HOUR_MS)
}

// One timer for the whole sidebar, not one per card: subscribers are woken
// together at the earliest moment any of their readouts changes. A card
// counting minutes therefore costs nothing while the card beside it is still
// counting seconds, and the last unsubscribe leaves no timer behind.
const subscribers = new Map<(now: number) => void, number>()
let timer: ReturnType<typeof setTimeout> | undefined

function schedule(): void {
  clearTimeout(timer)
  if (subscribers.size === 0) {
    return
  }
  const now = Date.now()
  let delay = HOUR_MS
  for (const since of subscribers.values()) {
    delay = Math.min(delay, nextChangeMs(now - since))
  }
  timer = setTimeout(() => {
    const at = Date.now()
    for (const notify of subscribers.keys()) {
      notify(at)
    }
    schedule()
  }, delay)
}

// subscribeAge calls notify with the current wall clock every time the readout
// for a status that started at `since` changes. Returns its unsubscribe.
export function subscribeAge(since: number, notify: (now: number) => void): () => void {
  subscribers.set(notify, since)
  schedule()
  return () => {
    subscribers.delete(notify)
    schedule()
  }
}
