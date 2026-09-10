// The four times a prompt can be parked for, and the words the card says about
// one that is parked. Kept out of the components because both of them — the
// dialog that offers the times and the card that counts one down — need the
// same arithmetic, and because it is the half of this feature that can be
// tested without a DOM.

/** One offer in the dialog: a name, and the moment picking it resolves to. */
export interface ScheduleChoice {
  label: string
  /** Unix seconds, which is what the store keeps and the backend compares. */
  at: number
}

const MINUTE = 60
const HOUR = 60 * MINUTE

// The hour a prompt left for "Tomorrow" lands on. Morning rather than midnight:
// what is parked overnight is picked up at the start of a working day, and a
// prompt typed at a session at 00:00 is one nobody is there to read.
const TOMORROW_HOUR = 9

/**
 * The times on offer, resolved against now (a Date, so a test can stand still).
 *
 * Tomorrow is tomorrow's date at TOMORROW_HOUR, never today's — at 3am "in the
 * morning" and "tomorrow morning" are different days, and the button says
 * tomorrow. Every choice carries the clock time it resolves to, so the label
 * never has to be trusted.
 */
export function scheduleChoices(now: Date): ScheduleChoice[] {
  const seconds = Math.floor(now.getTime() / 1000)
  const tomorrow = new Date(now)
  tomorrow.setDate(tomorrow.getDate() + 1)
  tomorrow.setHours(TOMORROW_HOUR, 0, 0, 0)
  return [
    { label: "15 min", at: seconds + 15 * MINUTE },
    { label: "1 hour", at: seconds + HOUR },
    { label: "4 hours", at: seconds + 4 * HOUR },
    { label: "Tomorrow", at: Math.floor(tomorrow.getTime() / 1000) },
  ]
}

/** The wall clock a choice lands on, in the user's own locale — "09:00". */
export function clockAt(at: number): string {
  return new Date(at * 1000).toLocaleTimeString(undefined, {
    hour: "2-digit",
    minute: "2-digit",
  })
}

/**
 * How far off a scheduled prompt is, for the rung on the card: "in 45m", "in
 * 3h", "in 2d". Null once it is due — the card has something else to say then,
 * because a prompt that is due and still parked is one waiting for a prompt to
 * be typed at.
 */
export function timeUntil(at: number, now: Date): string | null {
  const seconds = at - Math.floor(now.getTime() / 1000)
  if (seconds <= 0) {
    return null
  }
  if (seconds < HOUR) {
    // Rounded up, so a prompt 30 seconds out says "in 1m" rather than "in 0m".
    return `in ${Math.ceil(seconds / MINUTE)}m`
  }
  if (seconds < 24 * HOUR) {
    return `in ${Math.round(seconds / HOUR)}h`
  }
  return `in ${Math.round(seconds / (24 * HOUR))}d`
}

/**
 * When a scheduled prompt lands, spelled for the tooltip: the clock alone for
 * today, the weekday in front of it for any other day. The date is what the
 * countdown on the card cannot say — "in 2d" is not a time anyone can plan
 * around.
 */
export function scheduledFor(at: number, now: Date): string {
  const when = new Date(at * 1000)
  const clock = clockAt(at)
  if (when.toDateString() === now.toDateString()) {
    return `today ${clock}`
  }
  return `${when.toLocaleDateString(undefined, { weekday: "short" })} ${clock}`
}
