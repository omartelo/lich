import { describe, expect, it } from "vitest"
import { missingKeySequence, type TermKeyState } from "./term-keys"

const key = (over: Partial<TermKeyState>): TermKeyState => ({
  ctrlKey: false,
  metaKey: false,
  shiftKey: false,
  altKey: false,
  key: "",
  ...over,
})

describe("missingKeySequence", () => {
  it("maps Shift+Tab to backtab (CSI Z)", () => {
    expect(missingKeySequence(key({ shiftKey: true, key: "Tab" }))).toBe("\x1b[Z")
  })

  it("maps WebKitGTK's ISO_Left_Tab keysym to backtab via event.code", () => {
    const gtk = { ...key({ shiftKey: true, key: "ISO_Left_Tab" }), code: "Tab" }
    expect(missingKeySequence(gtk)).toBe("\x1b[Z")
  })

  it("prefixes Alt+<char> with ESC", () => {
    expect(missingKeySequence(key({ altKey: true, key: "t" }))).toBe("\x1bt")
    expect(missingKeySequence(key({ altKey: true, shiftKey: true, key: "T" }))).toBe("\x1bT")
  })

  it("maps Alt+Backspace to ESC DEL", () => {
    expect(missingKeySequence(key({ altKey: true, key: "Backspace" }))).toBe("\x1b\x7f")
  })

  it("leaves keys the terminal already handles alone", () => {
    expect(missingKeySequence(key({ key: "Tab" }))).toBeNull() // plain tab
    expect(missingKeySequence(key({ key: "t" }))).toBeNull() // plain char
    expect(missingKeySequence(key({ ctrlKey: true, shiftKey: true, key: "Tab" }))).toBeNull()
    expect(missingKeySequence(key({ metaKey: true, altKey: true, key: "t" }))).toBeNull()
    expect(missingKeySequence(key({ altKey: true, key: "ArrowLeft" }))).toBeNull() // non-char
  })

  it("ignores AltGr composition", () => {
    const altGr = {
      ...key({ altKey: true, key: "ł" }),
      getModifierState: (k: string) => k === "AltGraph",
    }
    expect(missingKeySequence(altGr)).toBeNull()
  })
})
