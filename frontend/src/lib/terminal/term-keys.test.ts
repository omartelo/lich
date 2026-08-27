import { describe, expect, it } from "vitest"
import { defaultHotkeys, type KeyState } from "../hotkeys"
import { chordSequence, shouldOpenTerminalSearch } from "./term-keys"

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
  it("translates Ctrl+Backspace and rejects extra modifiers", () => {
    const hotkeys = defaultHotkeys()
    expect(chordSequence(key({ ctrlKey: true, key: "Backspace" }), hotkeys, false, false)).toBe(
      "\x17",
    )
    expect(
      chordSequence(
        key({ ctrlKey: true, shiftKey: true, key: "Backspace" }),
        hotkeys,
        false,
        false,
      ),
    ).toBeNull()
    expect(
      chordSequence(key({ ctrlKey: true, altKey: true, key: "Backspace" }), hotkeys, false, false),
    ).toBeNull()
  })

  it("translates Ctrl+V case-insensitively outside Windows", () => {
    const hotkeys = defaultHotkeys()
    expect(chordSequence(key({ ctrlKey: true, key: "v" }), hotkeys, false, false)).toBe("\x16")
    expect(chordSequence(key({ ctrlKey: true, key: "V" }), hotkeys, false, false)).toBe("\x16")
  })

  it("translates the same Ctrl+V binding to ESC v on Windows", () => {
    const hotkeys = defaultHotkeys()
    expect(chordSequence(key({ ctrlKey: true, key: "v" }), hotkeys, false, true)).toBe("\x1bv")
    expect(chordSequence(key({ altKey: true, key: "v" }), hotkeys, false, true)).toBeNull()
    expect(
      chordSequence(key({ ctrlKey: true, shiftKey: true, key: "v" }), hotkeys, false, true),
    ).toBeNull()
  })

  it("translates Shift+Enter and rejects extra modifiers", () => {
    const hotkeys = defaultHotkeys()
    expect(chordSequence(key({ shiftKey: true, key: "Enter" }), hotkeys, false, false)).toBe(
      "\x1b\r",
    )
    expect(
      chordSequence(key({ ctrlKey: true, shiftKey: true, key: "Enter" }), hotkeys, false, false),
    ).toBeNull()
    expect(
      chordSequence(key({ altKey: true, shiftKey: true, key: "Enter" }), hotkeys, false, false),
    ).toBeNull()
  })

  it("continues translating repeated PTY keydowns", () => {
    const hotkeys = defaultHotkeys()
    expect(
      chordSequence(key({ ctrlKey: true, key: "Backspace", repeat: true }), hotkeys, false, false),
    ).toBe("\x17")
  })

  it("uses literal Ctrl rather than Cmd for terminal defaults on macOS", () => {
    const hotkeys = defaultHotkeys()
    expect(chordSequence(key({ ctrlKey: true, key: "v" }), hotkeys, true, false)).toBe("\x16")
    expect(chordSequence(key({ metaKey: true, key: "v" }), hotkeys, true, false)).toBeNull()
  })

  it("observes a rebind and releases the old chord", () => {
    const hotkeys = {
      ...defaultHotkeys(),
      eraseTerminalWord: { mod: false, ctrl: false, shift: false, alt: true, key: "e" },
    }
    expect(chordSequence(key({ altKey: true, key: "e" }), hotkeys, false, false)).toBe("\x17")
    expect(
      chordSequence(key({ ctrlKey: true, key: "Backspace" }), hotkeys, false, false),
    ).toBeNull()
  })

  it("does not translate an unassigned action", () => {
    const hotkeys = { ...defaultHotkeys(), insertTerminalNewline: null }
    expect(chordSequence(key({ shiftKey: true, key: "Enter" }), hotkeys, false, false)).toBeNull()
  })
})

describe("shouldOpenTerminalSearch", () => {
  const target = (tagName: string, { editable = false, inTerminal = false } = {}): EventTarget =>
    ({
      tagName,
      isContentEditable: editable,
      closest: (selector: string) => (selector === ".xterm" && inTerminal ? {} : null),
    }) as unknown as EventTarget

  it("matches, rebinds, and disables terminal search", () => {
    const defaults = defaultHotkeys()
    expect(
      shouldOpenTerminalSearch(
        key({ ctrlKey: true, key: "f" }),
        defaults.terminalSearch,
        null,
        false,
      ),
    ).toBe(true)
    const rebound = { mod: false, ctrl: false, shift: false, alt: true, key: "f" }
    expect(shouldOpenTerminalSearch(key({ altKey: true, key: "f" }), rebound, null, false)).toBe(
      true,
    )
    expect(shouldOpenTerminalSearch(key({ ctrlKey: true, key: "f" }), rebound, null, false)).toBe(
      false,
    )
    expect(shouldOpenTerminalSearch(key({ ctrlKey: true, key: "f" }), null, null, false)).toBe(
      false,
    )
  })

  it("does not claim editable targets outside the terminal", () => {
    const binding = defaultHotkeys().terminalSearch
    const press = key({ ctrlKey: true, key: "f" })
    expect(shouldOpenTerminalSearch(press, binding, target("INPUT"), false)).toBe(false)
    expect(shouldOpenTerminalSearch(press, binding, target("TEXTAREA"), false)).toBe(false)
    expect(shouldOpenTerminalSearch(press, binding, target("DIV", { editable: true }), false)).toBe(
      false,
    )
  })

  it("keeps Ctrl+F active in xterm's editable textarea", () => {
    expect(
      shouldOpenTerminalSearch(
        key({ ctrlKey: true, key: "f" }),
        defaultHotkeys().terminalSearch,
        target("TEXTAREA", { inTerminal: true }),
        false,
      ),
    ).toBe(true)
  })
})
