import {describe, expect, it} from "vitest"
import {chordSequence, isSearchOpenChord, type TermKeyState} from "./term-keys"

function key(overrides: Partial<TermKeyState>): TermKeyState {
  return {ctrlKey: false, metaKey: false, shiftKey: false, altKey: false, key: "", ...overrides}
}

describe("chordSequence", () => {
  it("maps Ctrl+Backspace to ETB (erase word)", () => {
    expect(chordSequence(key({ctrlKey: true, key: "Backspace"}))).toBe("\x17")
  })

  it("leaves plain and modified Backspace to xterm", () => {
    expect(chordSequence(key({key: "Backspace"}))).toBeNull()
    expect(chordSequence(key({ctrlKey: true, shiftKey: true, key: "Backspace"}))).toBeNull()
    expect(chordSequence(key({altKey: true, key: "Backspace"}))).toBeNull()
  })

  it("maps Ctrl+V to SYN so TUIs see the keypress (Linux/macOS)", () => {
    expect(chordSequence(key({ctrlKey: true, key: "v", code: "KeyV"}))).toBe("\x16")
    expect(chordSequence(key({ctrlKey: true, key: "V", code: "KeyV"}))).toBe("\x16")
  })

  it("maps Ctrl+V to Alt+V (ESC+v) on Windows — Claude Code's image-paste binding there", () => {
    expect(chordSequence(key({ctrlKey: true, key: "v", code: "KeyV"}), true)).toBe("\x1bv")
    expect(chordSequence(key({ctrlKey: true, key: "V", code: "KeyV"}), true)).toBe("\x1bv")
  })

  it("leaves Ctrl+Shift+V (native text paste) alone", () => {
    expect(chordSequence(key({ctrlKey: true, shiftKey: true, key: "V", code: "KeyV"}))).toBeNull()
  })

  it("maps Shift+Enter to ESC+CR (insert newline)", () => {
    expect(chordSequence(key({shiftKey: true, key: "Enter"}))).toBe("\x1b\r")
  })

  it("leaves Enter with other modifiers alone", () => {
    expect(chordSequence(key({key: "Enter"}))).toBeNull()
    expect(chordSequence(key({ctrlKey: true, shiftKey: true, key: "Enter"}))).toBeNull()
    expect(chordSequence(key({altKey: true, shiftKey: true, key: "Enter"}))).toBeNull()
  })

  it("ignores unrelated keys", () => {
    expect(chordSequence(key({ctrlKey: true, key: "c", code: "KeyC"}))).toBeNull()
    expect(chordSequence(key({key: "a", code: "KeyA"}))).toBeNull()
  })
})

describe("isSearchOpenChord", () => {
  it("matches Ctrl+F (either case)", () => {
    expect(isSearchOpenChord(key({ctrlKey: true, key: "f"}))).toBe(true)
    expect(isSearchOpenChord(key({ctrlKey: true, key: "F"}))).toBe(true)
  })

  it("rejects Ctrl+F carrying another modifier", () => {
    expect(isSearchOpenChord(key({ctrlKey: true, shiftKey: true, key: "f"}))).toBe(false)
    expect(isSearchOpenChord(key({ctrlKey: true, altKey: true, key: "f"}))).toBe(false)
    expect(isSearchOpenChord(key({ctrlKey: true, metaKey: true, key: "f"}))).toBe(false)
  })

  it("rejects F without Ctrl and other Ctrl chords", () => {
    expect(isSearchOpenChord(key({key: "f"}))).toBe(false)
    expect(isSearchOpenChord(key({metaKey: true, key: "f"}))).toBe(false)
    expect(isSearchOpenChord(key({ctrlKey: true, key: "g"}))).toBe(false)
  })
})
