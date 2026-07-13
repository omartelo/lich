import { describe, expect, it } from "vitest"
import {
  altScreenWheelSequence,
  isTextPasteChord,
  missingKeySequence,
  sgrWheelSequence,
  type TermKeyState,
} from "./term-keys"

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

  it("maps Shift+Enter to ESC CR (newline in TUIs)", () => {
    expect(missingKeySequence(key({ shiftKey: true, key: "Enter" }))).toBe("\x1b\r")
  })

  it("maps Ctrl+Backspace to ETB (delete word)", () => {
    expect(missingKeySequence(key({ ctrlKey: true, key: "Backspace" }))).toBe("\x17")
    expect(
      missingKeySequence(key({ ctrlKey: true, shiftKey: true, key: "Backspace" })),
    ).toBeNull()
  })

  it("maps Ctrl+V to SYN so TUIs see the paste chord", () => {
    expect(missingKeySequence(key({ ctrlKey: true, key: "v" }))).toBe("\x16")
    expect(missingKeySequence(key({ ctrlKey: true, shiftKey: true, key: "V" }))).toBeNull()
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
    expect(missingKeySequence(key({ key: "Enter" }))).toBeNull() // plain enter
    expect(missingKeySequence(key({ ctrlKey: true, shiftKey: true, key: "Tab" }))).toBeNull()
    expect(missingKeySequence(key({ metaKey: true, altKey: true, key: "t" }))).toBeNull()
    expect(missingKeySequence(key({ altKey: true, key: "ArrowLeft" }))).toBeNull() // non-char
  })

  it("detects Ctrl+Shift+V as the text-paste chord", () => {
    expect(isTextPasteChord(key({ ctrlKey: true, shiftKey: true, key: "V" }))).toBe(true)
    const gtk = { ...key({ ctrlKey: true, shiftKey: true, key: "V" }), code: "KeyV" }
    expect(isTextPasteChord(gtk)).toBe(true)
    expect(isTextPasteChord(key({ ctrlKey: true, key: "v" }))).toBe(false)
    expect(isTextPasteChord(key({ ctrlKey: true, shiftKey: true, altKey: true, key: "V" }))).toBe(
      false,
    )
  })

  it("encodes the wheel as an SGR mouse report for mouse-tracking apps", () => {
    expect(sgrWheelSequence(120, 4, 9)).toBe("\x1b[<65;4;9M") // down → button 65
    expect(sgrWheelSequence(-3, 1, 1)).toBe("\x1b[<64;1;1M") // up → button 64
    expect(sgrWheelSequence(0, 4, 9)).toBeNull() // pure horizontal wheel
  })

  it("maps the wheel to PgDn/PgUp in the alternate screen", () => {
    expect(altScreenWheelSequence(120)).toBe("\x1b[6~") // down → PgDn
    expect(altScreenWheelSequence(-3)).toBe("\x1b[5~") // up → PgUp
    expect(altScreenWheelSequence(0)).toBeNull() // pure horizontal wheel
  })

  it("ignores AltGr composition", () => {
    const altGr = {
      ...key({ altKey: true, key: "ł" }),
      getModifierState: (k: string) => k === "AltGraph",
    }
    expect(missingKeySequence(altGr)).toBeNull()
  })
})
