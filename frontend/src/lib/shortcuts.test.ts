import { describe, expect, it } from "vitest"
import { DEFAULT_HOTKEYS, HOTKEY_ACTIONS, HOTKEY_GROUPS, UNASSIGNED } from "./hotkeys"
import { PASSTHROUGH_TITLE, shortcutGroups, TERMINAL_TITLE } from "./shortcuts"

const groupTitled = (
  groups: { title: string; rows: { label: string; keys: string }[] }[],
  title: string,
) => {
  const group = groups.find((g) => g.title === title)
  if (!group) throw new Error(`no group titled ${title}`)
  return group
}
const keysFor = (
  groups: { title: string; rows: { label: string; keys: string }[] }[],
  label: string,
) => groups.flatMap((g) => g.rows).find((row) => row.label === label)?.keys

describe("shortcutGroups", () => {
  it("lists every rebindable action, grouped the way the actions declare", () => {
    const groups = shortcutGroups(DEFAULT_HOTKEYS, false, false)
    expect(groups.map((g) => g.title)).toEqual([
      ...HOTKEY_GROUPS.map((g) => g.label),
      TERMINAL_TITLE,
      PASSTHROUGH_TITLE,
    ])
    const listed = groups.slice(0, -2).flatMap((g) => g.rows.map((row) => row.label))
    expect(listed).toEqual(HOTKEY_ACTIONS.map((a) => a.label))
    expect(groupTitled(groups, "Sessions").rows.map((r) => r.label)).toContain("New session")
  })

  it("formats each binding for the platform", () => {
    expect(keysFor(shortcutGroups(DEFAULT_HOTKEYS, false, false), "Command palette")).toBe("Ctrl+K")
    expect(keysFor(shortcutGroups(DEFAULT_HOTKEYS, true, false), "Command palette")).toBe("⌘K")
  })

  it("shows the user's binding, not the default", () => {
    const rebound = {
      ...DEFAULT_HOTKEYS,
      commandPalette: { mod: true, shift: false, alt: true, key: "p" },
    }
    expect(keysFor(shortcutGroups(rebound, false, false), "Command palette")).toBe("Ctrl+Alt+P")
  })

  // The sheet answers "what fires this right now"; listing the default an
  // action no longer holds would name a chord that does nothing.
  it("reads an unassigned action as unassigned rather than as its default", () => {
    const cleared = { ...DEFAULT_HOTKEYS, commandPalette: UNASSIGNED }
    expect(keysFor(shortcutGroups(cleared, false, false), "Command palette")).toBe("Unassigned")
  })

  it("lists the terminal search, which is bound but not rebindable", () => {
    expect(
      keysFor(shortcutGroups(DEFAULT_HOTKEYS, true, false), "Search the session's output"),
    ).toBe("Ctrl+F")
  })

  it("splits the image-paste chord by platform", () => {
    const on = (isWindows: boolean) => shortcutGroups(DEFAULT_HOTKEYS, false, isWindows)
    expect(keysFor(on(false), "Attach an image from the clipboard")).toBe("Ctrl+V")
    expect(keysFor(on(true), "Attach an image from the clipboard")).toBe("Alt+V")
  })

  it("keeps the platform-independent chords as Control, macOS included", () => {
    const groups = shortcutGroups(DEFAULT_HOTKEYS, true, false)
    expect(keysFor(groups, "Erase the previous word")).toBe("Ctrl+Backspace")
    expect(keysFor(groups, "Insert a newline without sending")).toBe("Shift+Enter")
  })
})
