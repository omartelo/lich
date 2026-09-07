import { describe, expect, it } from "vitest"
import {
  cursorShapeSequence,
  cursorVisibilitySequence,
  linkClickIsOurs,
  mouseEncodingSequence,
} from "./term-modes"

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

describe("cursorVisibilitySequence", () => {
  it("hides the cursor again for a TUI that had turned it off", () => {
    expect(cursorVisibilitySequence(true)).toBe("\x1b[?25l")
  })

  it("writes nothing when it was visible, which a fresh terminal already is", () => {
    expect(cursorVisibilitySequence(false)).toBe("")
  })
})

describe("cursorShapeSequence", () => {
  it("carries a bar cursor across the serialize/restore cycle", () => {
    expect(cursorShapeSequence({ style: "bar", blink: false })).toBe("\x1b[6 q")
    expect(cursorShapeSequence({ style: "bar", blink: true })).toBe("\x1b[5 q")
  })

  it("restores the other shapes DECSCUSR can select", () => {
    expect(cursorShapeSequence({ style: "block", blink: true })).toBe("\x1b[1 q")
    expect(cursorShapeSequence({ style: "block", blink: false })).toBe("\x1b[2 q")
    expect(cursorShapeSequence({ style: "underline", blink: true })).toBe("\x1b[3 q")
    expect(cursorShapeSequence({ style: "underline", blink: false })).toBe("\x1b[4 q")
  })

  it("writes nothing once the app resets the cursor (Ps 0, or a full reset)", () => {
    expect(cursorShapeSequence({})).toBe("")
    expect(cursorShapeSequence({ style: undefined, blink: undefined })).toBe("")
  })

  it("writes nothing for a shape name xterm added since", () => {
    expect(cursorShapeSequence({ style: "SOMETHING_NEW", blink: true })).toBe("")
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
