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
  it("opens the link on Ctrl+Click, and on Cmd+Click for macOS", () => {
    expect(linkClickIsOurs({ ctrlKey: true, metaKey: false })).toBe(true)
    expect(linkClickIsOurs({ ctrlKey: false, metaKey: true })).toBe(true)
  })

  it("ignores a plain click, which selects rather than opens", () => {
    expect(linkClickIsOurs({ ctrlKey: false, metaKey: false })).toBe(false)
  })
})
