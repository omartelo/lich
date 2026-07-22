import { describe, expect, it } from "vitest"
import {
  clampTerminalFontSize,
  clampZoom,
  DEFAULT_TERMINAL_FONT_SIZE,
  DEFAULT_ZOOM,
  TERMINAL_FONT_SIZE_MAX,
  TERMINAL_FONT_SIZE_MIN,
  ZOOM_MAX,
  ZOOM_MIN,
} from "./settings"

describe("clampZoom", () => {
  it("keeps a value inside the range", () => {
    expect(clampZoom(DEFAULT_ZOOM)).toBe(DEFAULT_ZOOM)
    expect(clampZoom(1.3)).toBe(1.3)
  })

  it("bounds both ends", () => {
    expect(clampZoom(ZOOM_MIN - 1)).toBe(ZOOM_MIN)
    expect(clampZoom(ZOOM_MAX + 1)).toBe(ZOOM_MAX)
  })

  // Repeated ±0.1 steps otherwise drift into 0.7000000000000001 and show as a
  // wrong percentage.
  it("snaps to one decimal so stepping does not drift", () => {
    expect(clampZoom(0.1 + 0.2 + 0.4)).toBe(0.7)
    expect(clampZoom(1.2000000000000002)).toBe(1.2)
  })
})

describe("clampTerminalFontSize", () => {
  it("keeps a value inside the range", () => {
    expect(clampTerminalFontSize(DEFAULT_TERMINAL_FONT_SIZE)).toBe(DEFAULT_TERMINAL_FONT_SIZE)
    expect(clampTerminalFontSize(20)).toBe(20)
  })

  it("bounds both ends", () => {
    expect(clampTerminalFontSize(TERMINAL_FONT_SIZE_MIN - 5)).toBe(TERMINAL_FONT_SIZE_MIN)
    expect(clampTerminalFontSize(TERMINAL_FONT_SIZE_MAX + 5)).toBe(TERMINAL_FONT_SIZE_MAX)
  })

  // xterm takes a number, and a fractional cell size makes the grid maths
  // land between pixels — keep it whole.
  it("rounds to a whole pixel", () => {
    expect(clampTerminalFontSize(14.4)).toBe(14)
    expect(clampTerminalFontSize(14.6)).toBe(15)
  })
})
