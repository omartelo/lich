import { describe, expect, it } from "vitest"
import { formatHandsOn, spellHandsOn } from "./hands-on"

describe("formatHandsOn", () => {
  // Under a minute is not a small number, it is no number: the store is up to a
  // flush behind, and "0m" reads as a readout that broke rather than as a
  // session that just opened.
  it("shows nothing below a minute", () => {
    expect(formatHandsOn(0)).toBe("")
    expect(formatHandsOn(59)).toBe("")
  })

  it("starts at the first whole minute", () => {
    expect(formatHandsOn(60)).toBe("1m")
    expect(formatHandsOn(119)).toBe("1m")
  })

  it("counts minutes up to the hour", () => {
    expect(formatHandsOn(48 * 60)).toBe("48m")
    expect(formatHandsOn(59 * 60 + 59)).toBe("59m")
  })

  // Zero-padded past the hour: "1h5m" and "1h50m" differ by one glyph in a
  // strip nobody stops to parse.
  it("pads the minutes once there are hours in front of them", () => {
    expect(formatHandsOn(3600)).toBe("1h00m")
    expect(formatHandsOn(3600 + 5 * 60)).toBe("1h05m")
    expect(formatHandsOn(3600 + 12 * 60)).toBe("1h12m")
  })

  it("keeps counting hours rather than rolling into days", () => {
    expect(formatHandsOn(14 * 3600 + 5 * 60)).toBe("14h05m")
    expect(formatHandsOn(100 * 3600)).toBe("100h00m")
  })

  // The backend answers 0 on every miss, but the RPC layer is typed, not
  // validated: a garbage number must render as nothing, never as "NaNm".
  it("renders nothing for a figure that is not a number", () => {
    expect(formatHandsOn(Number.NaN)).toBe("")
    expect(formatHandsOn(Number.POSITIVE_INFINITY)).toBe("")
    expect(formatHandsOn(-60)).toBe("")
  })
})

describe("spellHandsOn", () => {
  it("gives the tooltip the space the strip cannot spare", () => {
    expect(spellHandsOn(3600 + 12 * 60)).toBe("1h 12m")
    expect(spellHandsOn(48 * 60)).toBe("48m")
    expect(spellHandsOn(30)).toBe("")
  })
})
