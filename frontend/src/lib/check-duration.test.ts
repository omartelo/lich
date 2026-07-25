import { describe, expect, it } from "vitest"
import { checkDuration } from "./check-duration"

const start = "2026-07-25T10:00:00Z"
const at = (iso: string) => Date.parse(iso)

describe("checkDuration", () => {
  it("reports the span of a finished check", () => {
    expect(checkDuration(start, "2026-07-25T10:00:42Z")).toBe("42s")
    expect(checkDuration(start, "2026-07-25T10:02:30Z")).toBe("2m 30s")
    expect(checkDuration(start, "2026-07-25T10:05:00Z")).toBe("5m")
    expect(checkDuration(start, "2026-07-25T11:30:00Z")).toBe("1h 30m")
    expect(checkDuration(start, "2026-07-25T12:00:00Z")).toBe("2h")
  })

  // A check that is still running has no completedAt; its duration is measured
  // against now, which is what tells a waiting reviewer it has been stuck.
  it("measures a running check against the clock", () => {
    expect(checkDuration(start, "", at("2026-07-25T10:01:15Z"))).toBe("1m 15s")
  })

  it("returns nothing when it cannot tell", () => {
    expect(checkDuration("", "")).toBe("")
    expect(checkDuration("not a date", "")).toBe("")
    expect(checkDuration(start, "not a date")).toBe("")
  })

  // Clock skew between the forge and this machine can put "now" behind the
  // start; a negative duration is worse than none.
  it("returns nothing rather than a negative span", () => {
    expect(checkDuration(start, "", at("2026-07-25T09:59:00Z"))).toBe("")
  })
})
