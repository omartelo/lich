import { describe, expect, it } from "vitest"
import { zoomIntent, type ZoomKeyState } from "./zoom-keys"

const press = (over: Partial<ZoomKeyState>): ZoomKeyState => ({
  ctrlKey: false,
  metaKey: false,
  shiftKey: false,
  altKey: false,
  code: "",
  key: "",
  ...over,
})

describe("zoomIntent", () => {
  it("reads Ctrl+Shift+Equal as zoom in", () => {
    expect(zoomIntent(press({ ctrlKey: true, shiftKey: true, code: "Equal" }))).toBe("in")
  })

  it("reads unshifted Ctrl+Equal as zoom in too", () => {
    expect(zoomIntent(press({ ctrlKey: true, code: "Equal" }))).toBe("in")
  })

  it("reads Ctrl+Minus and Ctrl+Digit0", () => {
    expect(zoomIntent(press({ ctrlKey: true, code: "Minus" }))).toBe("out")
    expect(zoomIntent(press({ ctrlKey: true, code: "Digit0" }))).toBe("reset")
  })

  it("covers the numpad without Shift", () => {
    expect(zoomIntent(press({ ctrlKey: true, code: "NumpadAdd" }))).toBe("in")
    expect(zoomIntent(press({ ctrlKey: true, code: "NumpadSubtract" }))).toBe("out")
    expect(zoomIntent(press({ ctrlKey: true, code: "Numpad0" }))).toBe("reset")
  })

  it("falls back to the character on layouts with dedicated plus and minus keys", () => {
    expect(zoomIntent(press({ ctrlKey: true, key: "+", code: "BracketRight" }))).toBe("in")
    expect(zoomIntent(press({ ctrlKey: true, key: "-", code: "Slash" }))).toBe("out")
  })

  it("accepts Cmd as the primary modifier", () => {
    expect(zoomIntent(press({ metaKey: true, code: "Minus" }))).toBe("out")
  })

  it("ignores the chord without a primary modifier", () => {
    expect(zoomIntent(press({ code: "Equal" }))).toBeNull()
    expect(zoomIntent(press({ shiftKey: true, code: "Equal" }))).toBeNull()
  })

  it("ignores Alt so Alt chords still reach the PTY", () => {
    expect(zoomIntent(press({ ctrlKey: true, altKey: true, code: "Minus" }))).toBeNull()
  })

  it("does not claim shifted chords other than Equal", () => {
    expect(zoomIntent(press({ ctrlKey: true, shiftKey: true, code: "Minus" }))).toBeNull()
    expect(zoomIntent(press({ ctrlKey: true, shiftKey: true, code: "Digit0" }))).toBeNull()
    expect(zoomIntent(press({ ctrlKey: true, shiftKey: true, code: "NumpadAdd" }))).toBeNull()
  })

  it("ignores unrelated keys", () => {
    expect(zoomIntent(press({ ctrlKey: true, code: "KeyK" }))).toBeNull()
    expect(zoomIntent(press({ ctrlKey: true, code: "Digit1" }))).toBeNull()
  })
})
