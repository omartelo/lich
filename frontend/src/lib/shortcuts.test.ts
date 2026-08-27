import { describe, expect, it } from "vitest"
import { DEFAULT_HOTKEYS, HOTKEY_ACTIONS, HOTKEY_GROUPS } from "./hotkeys"
import { shortcutGroups, type ShortcutGroup } from "./shortcuts"

function keysFor(groups: ShortcutGroup[], label: string): string | undefined {
  return groups.flatMap((group) => group.rows).find((row) => row.label === label)?.keys
}

describe("shortcutGroups", () => {
  it("lists every action once under its declared group", () => {
    const groups = shortcutGroups(DEFAULT_HOTKEYS, false)
    expect(groups.map((group) => group.title)).toEqual(HOTKEY_GROUPS.map((group) => group.label))
    expect(groups.flatMap((group) => group.rows).map((row) => row.label)).toEqual(
      HOTKEY_ACTIONS.map((action) => action.label),
    )
  })

  it("shows app, search, and terminal translation bindings without zoom", () => {
    const groups = shortcutGroups(DEFAULT_HOTKEYS, false)
    expect(keysFor(groups, "Command palette")).toBe("Ctrl+K")
    expect(keysFor(groups, "Search the session's output")).toBe("Ctrl+F")
    expect(keysFor(groups, "Attach an image from the clipboard")).toBe("Ctrl+V")
    expect(keysFor(groups, "Insert a newline without sending")).toBe("Shift+Enter")
    expect(keysFor(groups, "Erase the previous word")).toBe("Ctrl+Backspace")
    expect(keysFor(groups, "Zoom in")).toBeUndefined()
  })

  it("shows rebindings and disabled actions instead of fixed rows", () => {
    const groups = shortcutGroups(
      {
        ...DEFAULT_HOTKEYS,
        terminalSearch: null,
        insertTerminalNewline: { mod: false, ctrl: false, shift: false, alt: true, key: "n" },
      },
      false,
    )
    expect(keysFor(groups, "Search the session's output")).toBe("Unassigned")
    expect(keysFor(groups, "Insert a newline without sending")).toBe("Alt+N")
  })

  it("formats platform-primary and literal Control distinctly on macOS", () => {
    const groups = shortcutGroups(DEFAULT_HOTKEYS, true)
    expect(keysFor(groups, "Command palette")).toBe("⌘K")
    expect(keysFor(groups, "Search the session's output")).toBe("Ctrl+F")
  })
})
