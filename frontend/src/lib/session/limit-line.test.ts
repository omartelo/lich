import { describe, expect, it } from "vitest"
import { limitLine } from "./limit-line"
import { scheduledFor } from "./schedule"

const now = new Date(2026, 9, 8, 18, 0, 0)
const seconds = Math.floor(now.getTime() / 1000)
const HOUR = 3600

describe("limitLine", () => {
  it("counts down to the continuation parked after the reset", () => {
    const resetsAt = seconds + 2 * HOUR
    expect(limitLine({ window: "session", resetsAt }, resetsAt + 120, now)).toEqual({
      name: "Session limit",
      next: "resumes",
      when: "in 2h",
    })
  })

  it("says when it resets when nothing will resume it", () => {
    const resetsAt = seconds + 3 * 24 * HOUR
    expect(limitLine({ window: "weekly", resetsAt }, 0, now)).toEqual({
      name: "Weekly limit",
      next: "resets",
      when: scheduledFor(resetsAt, now),
    })
  })

  // A prompt the person parked before the reset is theirs, not the
  // continuation: it lands into a limit that still holds.
  it("does not call an earlier prompt a resume", () => {
    const resetsAt = seconds + 2 * HOUR
    expect(limitLine({ window: "", resetsAt }, seconds + HOUR, now).next).toBe("resets")
  })

  it("names only the limit once its reset is past or unknown", () => {
    expect(limitLine({ window: "", resetsAt: seconds - 60 }, 0, now)).toEqual({
      name: "Usage limit",
      next: "",
      when: "",
    })
    expect(limitLine({ window: "session", resetsAt: 0 }, seconds + HOUR, now).next).toBe("")
  })
})
