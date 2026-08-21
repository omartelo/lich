import { describe, expect, it } from "vitest"
import { defaultHotkeys, type KeyState } from "../hotkeys"
import { chordSequence, isSearchOpenChord } from "./term-keys"

const key = (over: Partial<KeyState>): KeyState => ({
  ctrlKey: false,
  metaKey: false,
  shiftKey: false,
  altKey: false,
  key: "",
  repeat: false,
  ...over,
})

describe("chordSequence", () => {
  it("writes the default terminal translations", () => {
    const hotkeys = defaultHotkeys(false)
    expect(chordSequence(key({ ctrlKey: true, key: "Backspace" }), hotkeys, false, false)).toBe(
      "\x17",
    )
    expect(chordSequence(key({ ctrlKey: true, key: "v" }), hotkeys, false, false)).toBe("\x16")
    expect(chordSequence(key({ shiftKey: true, key: "Enter" }), hotkeys, false, false)).toBe(
      "\x1b\r",
    )
  })

  it("uses literal Ctrl rather than Cmd for terminal defaults on macOS", () => {
    const hotkeys = defaultHotkeys(false)
    expect(chordSequence(key({ ctrlKey: true, key: "v" }), hotkeys, true, false)).toBe("\x16")
    expect(chordSequence(key({ metaKey: true, key: "v" }), hotkeys, true, false)).toBeNull()
  })

  it("uses Windows' Alt+V default and sequence", () => {
    const hotkeys = defaultHotkeys(true)
    expect(chordSequence(key({ altKey: true, key: "v" }), hotkeys, false, true)).toBe("\x1bv")
    expect(chordSequence(key({ ctrlKey: true, key: "v" }), hotkeys, false, true)).toBeNull()
  })

  it("observes a rebind and releases the old chord", () => {
    const hotkeys = {
      ...defaultHotkeys(false),
      eraseTerminalWord: { mod: false, ctrl: false, shift: false, alt: true, key: "e" },
    }
    expect(chordSequence(key({ altKey: true, key: "e" }), hotkeys, false, false)).toBe("\x17")
    expect(
      chordSequence(key({ ctrlKey: true, key: "Backspace" }), hotkeys, false, false),
    ).toBeNull()
  })

  it("does not translate an unassigned action", () => {
    const hotkeys = { ...defaultHotkeys(false), insertTerminalNewline: null }
    expect(chordSequence(key({ shiftKey: true, key: "Enter" }), hotkeys, false, false)).toBeNull()
  })
})

describe("isSearchOpenChord", () => {
  it("matches, rebinds, and disables terminal search", () => {
    const defaults = defaultHotkeys(false)
    expect(
      isSearchOpenChord(key({ ctrlKey: true, key: "f" }), defaults.terminalSearch, false),
    ).toBe(true)
    const rebound = { mod: false, ctrl: false, shift: false, alt: true, key: "f" }
    expect(isSearchOpenChord(key({ altKey: true, key: "f" }), rebound, false)).toBe(true)
    expect(isSearchOpenChord(key({ ctrlKey: true, key: "f" }), rebound, false)).toBe(false)
    expect(isSearchOpenChord(key({ ctrlKey: true, key: "f" }), null, false)).toBe(false)
  })
})
