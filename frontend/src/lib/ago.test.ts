import { describe, expect, it } from "vitest"
import { agoLabel, agoUnit } from "./ago"

const MIN = 60_000
const HOUR = 60 * MIN
const DAY = 24 * HOUR

describe("agoUnit", () => {
  it("floors each unit rather than rounding, so it never claims time that has not passed", () => {
    expect(agoUnit(59 * 1000)).toBe("now")
    expect(agoUnit(MIN)).toBe("1m")
    expect(agoUnit(59 * MIN + 59_000)).toBe("59m")
    expect(agoUnit(HOUR)).toBe("1h")
    expect(agoUnit(23 * HOUR + 59 * MIN)).toBe("23h")
    expect(agoUnit(DAY)).toBe("1d")
    expect(agoUnit(6 * DAY + 23 * HOUR)).toBe("6d")
    expect(agoUnit(7 * DAY)).toBe("1w")
    expect(agoUnit(21 * DAY)).toBe("3w")
  })

  it("reads a backwards clock jump as now, never as a negative age", () => {
    expect(agoUnit(-5 * HOUR)).toBe("now")
  })
})

describe("agoLabel", () => {
  const now = 1_700_000_000_000

  it("phrases the unit, so a history row cannot be read as a live card's age", () => {
    expect(agoLabel(now / 1000 - 2 * 3600, now)).toBe("2h ago")
    expect(agoLabel(now / 1000 - 3 * 86400, now)).toBe("3d ago")
    expect(agoLabel(now / 1000 - 21 * 86400, now)).toBe("3w ago")
  })

  it("says just now rather than 'now ago'", () => {
    expect(agoLabel(now / 1000 - 5, now)).toBe("just now")
  })

  it("has no date for a row parked before lich recorded one", () => {
    expect(agoLabel(0, now)).toBe("")
    expect(agoLabel(-1, now)).toBe("")
  })
})
