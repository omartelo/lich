import { describe, expect, it } from "vitest"
import { linkClickIsOurs, mouseEncodingSequence } from "./term-modes"

describe("mouseEncodingSequence", () => {
  it("restores SGR, the encoding every modern TUI selects", () => {
    expect(mouseEncodingSequence("SGR")).toBe("\x1b[?1006h")
  })

  it("restores the other encodings xterm can be in", () => {
    expect(mouseEncodingSequence("SGR_PIXELS")).toBe("\x1b[?1016h")
    expect(mouseEncodingSequence("UTF8")).toBe("\x1b[?1005h")
    expect(mouseEncodingSequence("URXVT")).toBe("\x1b[?1015h")
  })

  it("writes nothing for X10, which is what a fresh terminal already is", () => {
    expect(mouseEncodingSequence("DEFAULT")).toBe("")
  })

  it("writes nothing when the encoding could not be read or is unknown", () => {
    expect(mouseEncodingSequence(undefined)).toBe("")
    expect(mouseEncodingSequence("")).toBe("")
    expect(mouseEncodingSequence("SOMETHING_NEW")).toBe("")
  })
})

describe("linkClickIsOurs", () => {
  it("opens the link when no app is reading the mouse", () => {
    expect(linkClickIsOurs({ ctrlKey: true, metaKey: false }, "none")).toBe(true)
    expect(linkClickIsOurs({ ctrlKey: false, metaKey: true }, "none")).toBe(true)
  })

  it("stands down while an app reads the mouse — it got the same click", () => {
    for (const mode of ["x10", "vt200", "drag", "any"]) {
      expect(linkClickIsOurs({ ctrlKey: true, metaKey: false }, mode)).toBe(false)
    }
  })

  it("ignores a plain click, which selects rather than opens", () => {
    expect(linkClickIsOurs({ ctrlKey: false, metaKey: false }, "none")).toBe(false)
  })
})
