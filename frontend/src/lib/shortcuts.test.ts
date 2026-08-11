import { describe, expect, it } from "vitest"
import { DEFAULT_HOTKEYS, HOTKEY_ACTIONS } from "./hotkeys"
import { shortcutGroups } from "./shortcuts"

const rowFor = (group: { rows: { label: string; keys: string }[] }, label: string) =>
  group.rows.find((row) => row.label === label)

describe("shortcutGroups", () => {
  it("lists every rebindable action under lich", () => {
    const [lich] = shortcutGroups(DEFAULT_HOTKEYS, false, false)
    expect(lich.title).toBe("lich")
    expect(lich.rows.map((row) => row.label)).toEqual(HOTKEY_ACTIONS.map((a) => a.label))
    expect(rowFor(lich, "Command palette")?.keys).toBe("Ctrl+K")
  })

  it("shows the user's binding, not the default", () => {
    const rebound = {
      ...DEFAULT_HOTKEYS,
      commandPalette: { mod: true, shift: false, alt: true, key: "p" },
    }
    const [lich] = shortcutGroups(rebound, false, false)
    expect(rowFor(lich, "Command palette")?.keys).toBe("Ctrl+Alt+P")
  })

  it("formats the lich bindings for macOS", () => {
    const [lich] = shortcutGroups(DEFAULT_HOTKEYS, true, false)
    expect(rowFor(lich, "Command palette")?.keys).toBe("⌘K")
  })

  it("splits the image-paste chord by platform", () => {
    const agentOn = (isWindows: boolean) => shortcutGroups(DEFAULT_HOTKEYS, false, isWindows)[1]
    expect(rowFor(agentOn(false), "Attach an image from the clipboard")?.keys).toBe("Ctrl+V")
    expect(rowFor(agentOn(true), "Attach an image from the clipboard")?.keys).toBe("Alt+V")
  })

  it("keeps the platform-independent chords as Control, macOS included", () => {
    const agent = shortcutGroups(DEFAULT_HOTKEYS, true, false)[1]
    expect(agent.title).toBe("Passed through to the agent")
    expect(rowFor(agent, "Erase the previous word")?.keys).toBe("Ctrl+Backspace")
    expect(rowFor(agent, "Insert a newline without sending")?.keys).toBe("Shift+Enter")
  })
})
