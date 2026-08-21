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

  it("continues translating repeated PTY keydowns", () => {
    const hotkeys = defaultHotkeys(false)
    expect(
      chordSequence(key({ ctrlKey: true, key: "Backspace", repeat: true }), hotkeys, false, false),
    ).toBe("\x17")
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

describe("shouldOpenTerminalSearch", () => {
  const target = (tagName: string, { editable = false, inTerminal = false } = {}): EventTarget =>
    ({
      tagName,
      isContentEditable: editable,
      closest: (selector: string) => (selector === ".xterm" && inTerminal ? {} : null),
    }) as unknown as EventTarget

  it("matches, rebinds, and disables terminal search", () => {
    const defaults = defaultHotkeys(false)
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
    const binding = defaultHotkeys(false).terminalSearch
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
        defaultHotkeys(false).terminalSearch,
        target("TEXTAREA", { inTerminal: true }),
        false,
      ),
    ).toBe(true)
  })
})
