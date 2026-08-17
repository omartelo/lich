import { describe, expect, it } from "vitest"
import type { QuotaPlan } from "@/lib/api-types"
import { formatWindow, hottestWindow, shortWindow, timeLeft } from "./quota-format"

const plan = (...windows: Array<{ label: string; percent: number }>): QuotaPlan => ({
  provider: "claude",
  name: "Claude Code",
  status: "ok",
  windows: windows.map((w) => ({ ...w, seconds: 18000 })),
})

describe("hottestWindow", () => {
  it("picks the window closest to spent, whatever its position", () => {
    const got = hottestWindow(
      plan({ label: "Session", percent: 4 }, { label: "Weekly", percent: 84 }),
    )
    expect(got?.label).toBe("Weekly")
  })

  it("keeps the provider's order on a tie", () => {
    const got = hottestWindow(
      plan({ label: "Session", percent: 50 }, { label: "Weekly", percent: 50 }),
    )
    expect(got?.label).toBe("Session")
  })

  it("is null for a plan with no windows", () => {
    expect(hottestWindow(plan())).toBeNull()
    expect(hottestWindow({ provider: "codex", name: "Codex", status: "signed-out" })).toBeNull()
  })
})

describe("shortWindow", () => {
  it("names a length in the width a status bar has", () => {
    expect(shortWindow(18000)).toBe("5h")
    expect(shortWindow(604800)).toBe("wk")
    expect(shortWindow(2592000)).toBe("30d")
  })

  it("is empty when the provider reports no length", () => {
    expect(shortWindow(0)).toBe("")
    expect(shortWindow(-1)).toBe("")
  })
})

describe("formatWindow", () => {
  it("spells a week as days, where the bar has room", () => {
    expect(formatWindow(18000)).toBe("5h")
    expect(formatWindow(604800)).toBe("7d")
    expect(formatWindow(2592000)).toBe("30d")
    expect(formatWindow(0)).toBe("")
  })
})

describe("timeLeft", () => {
  const now = new Date("2026-08-17T12:00:00Z")

  it("uses the two coarsest units that apply", () => {
    expect(timeLeft("2026-08-24T08:00:00Z", now)).toBe("6d 20h")
    expect(timeLeft("2026-08-17T16:12:00Z", now)).toBe("4h 12m")
    expect(timeLeft("2026-08-17T12:41:00Z", now)).toBe("41m")
  })

  it("never counts down to a moment that has passed", () => {
    expect(timeLeft("2026-08-17T11:59:00Z", now)).toBe("")
    expect(timeLeft("2026-08-17T12:00:00Z", now)).toBe("")
  })

  it("rounds the last minute up rather than showing 0m", () => {
    expect(timeLeft("2026-08-17T12:00:30Z", now)).toBe("1m")
  })

  it("is empty without a reset time it can read", () => {
    expect(timeLeft(undefined, now)).toBe("")
    expect(timeLeft("", now)).toBe("")
    expect(timeLeft("whenever", now)).toBe("")
  })
})
