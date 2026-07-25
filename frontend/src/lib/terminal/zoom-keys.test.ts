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
  // The regression this whole module exists for: "Ctrl +" is physically
  // Ctrl+Shift+Equal on US and ABNT2 alike. Matching on the character made this
  // press miss, so Chromium's accelerator zoomed on top of the app's own zoom.
  it("reads Ctrl+Shift+Equal — the '+' a real keyboard types — as zoom in", () => {
    expect(zoomIntent(press({ ctrlKey: true, shiftKey: true, code: "Equal" }))).toBe("in")
  })

  it("reads unshifted Ctrl+Equal as zoom in too", () => {
    expect(zoomIntent(press({ ctrlKey: true, code: "Equal" }))).toBe("in")
  })

  it("reads Ctrl+Minus and Ctrl+Digit0", () => {
    expect(zoomIntent(press({ ctrlKey: true, code: "Minus" }))).toBe("out")
    expect(zoomIntent(press({ ctrlKey: true, code: "Digit0" }))).toBe("reset")
  })

  it("covers the numpad, where no layout needs Shift", () => {
    expect(zoomIntent(press({ ctrlKey: true, code: "NumpadAdd" }))).toBe("in")
    expect(zoomIntent(press({ ctrlKey: true, code: "NumpadSubtract" }))).toBe("out")
    expect(zoomIntent(press({ ctrlKey: true, code: "Numpad0" }))).toBe("reset")
  })

  it("falls back to the character on layouts whose +/− are dedicated keys", () => {
    // German: "+" is its own key (code BracketRight) and "-" sits on Slash —
    // neither code is in the table, but the character says what the press means.
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

  // Shift is what types "+", so it is only meaningful on Equal. Claiming
  // Ctrl+Shift+Minus / Ctrl+Shift+0 too would steal chords from the PTY for
  // nothing.
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
