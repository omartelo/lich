import { describe, expect, it } from "vitest"
import { clockAt, scheduleChoices, scheduledFor, timeUntil } from "./schedule"

// A Wednesday afternoon, so "tomorrow" is a different weekday from "today" and
// the two readouts cannot pass for each other.
const NOW = new Date(2026, 8, 2, 14, 30, 0)
const seconds = (date: Date) => Math.floor(date.getTime() / 1000)

describe("scheduleChoices", () => {
  it("offers the four times, each resolved against now", () => {
    const choices = scheduleChoices(NOW)
    expect(choices.map((c) => c.label)).toEqual(["15 min", "1 hour", "4 hours", "Tomorrow"])
    expect(choices[0].at).toBe(seconds(NOW) + 900)
    expect(choices[1].at).toBe(seconds(NOW) + 3600)
    expect(choices[2].at).toBe(seconds(NOW) + 4 * 3600)
    expect(choices[3].at).toBe(seconds(new Date(2026, 8, 3, 9, 0, 0)))
  })

  // The one case where the word could lie: in the small hours, 9am is still
  // today, and a button that says Tomorrow must not mean six hours from now.
  it("keeps Tomorrow on tomorrow's date at 3am", () => {
    const smallHours = new Date(2026, 8, 2, 3, 0, 0)
    expect(scheduleChoices(smallHours)[3].at).toBe(seconds(new Date(2026, 8, 3, 9, 0, 0)))
  })
})

describe("timeUntil", () => {
  it("counts down in the unit the distance calls for", () => {
    expect(timeUntil(seconds(NOW) + 30, NOW)).toBe("in 1m")
    expect(timeUntil(seconds(NOW) + 45 * 60, NOW)).toBe("in 45m")
    expect(timeUntil(seconds(NOW) + 3 * 3600, NOW)).toBe("in 3h")
    expect(timeUntil(seconds(NOW) + 47 * 3600, NOW)).toBe("in 2d")
  })

  it("says nothing about a prompt that is already due", () => {
    expect(timeUntil(seconds(NOW), NOW)).toBeNull()
    expect(timeUntil(seconds(NOW) - 3600, NOW)).toBeNull()
  })
})

describe("scheduledFor", () => {
  it("names the day only when it is not today", () => {
    const today = seconds(new Date(2026, 8, 2, 21, 0, 0))
    expect(scheduledFor(today, NOW)).toBe(`today ${clockAt(today)}`)

    const tomorrow = seconds(new Date(2026, 8, 3, 9, 0, 0))
    expect(scheduledFor(tomorrow, NOW)).not.toContain("today")
    expect(scheduledFor(tomorrow, NOW)).toContain(clockAt(tomorrow))
  })
})
