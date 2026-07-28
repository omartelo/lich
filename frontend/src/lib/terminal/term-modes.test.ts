import { describe, expect, it } from "vitest"
import { mouseEncodingSequence } from "./term-modes"

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
