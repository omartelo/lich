import { scheduledFor, timeUntil } from "./schedule"
import type { SessionLimit } from "./session-events"
import { t } from "@/lib/i18n/i18n"
import type { MessageKey } from "@/lib/i18n/locales/en"

const WINDOW_NAMES = {
  session: "session.limitLine.sessionWindow",
  weekly: "session.limitLine.weeklyWindow",
  "": "session.limitLine.usageWindow",
} as const satisfies Record<SessionLimit["window"], MessageKey>

// What the card says about a usage limit: the window's name, then what happens
// next. A prompt parked at or after the reset is the continuation (lich parks
// it there, internal/relay resume.go), so the card counts down to it; without
// one it says when the limit lifts, which is the moment a person can go on by
// hand. `when` is empty for a reset already behind us or one never reported.
export interface LimitLine {
  name: string
  next: "resumes" | "resets" | ""
  when: string
}

export function limitLine(limit: SessionLimit, scheduledAt: number, now: Date): LimitLine {
  const name = t(WINDOW_NAMES[limit.window])
  const resumesIn = scheduledAt >= limit.resetsAt ? timeUntil(scheduledAt, now) : null
  if (limit.resetsAt > 0 && resumesIn) {
    return { name, next: "resumes", when: resumesIn }
  }
  if (limit.resetsAt * 1000 > now.getTime()) {
    return { name, next: "resets", when: scheduledFor(limit.resetsAt, now) }
  }
  return { name, next: "", when: "" }
}
