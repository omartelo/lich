import { beforeEach, describe, expect, it, vi } from "vitest"
import {
  readSettingsQuery,
  readSettingsSection,
  writeSettingsQuery,
  writeSettingsSection,
} from "./settings-prefs"

// The suite runs in node, which has no localStorage. These are the prefs that
// have to survive leaving the screen, so the storage is stubbed and the round
// trip through it is what is checked.
const stored = new Map<string, string>()

vi.stubGlobal("localStorage", {
  getItem: (key: string) => stored.get(key) ?? null,
  setItem: (key: string, value: string) => {
    stored.set(key, value)
  },
  removeItem: (key: string) => {
    stored.delete(key)
  },
})

beforeEach(() => {
  stored.clear()
})

describe("the stored section", () => {
  it("opens on the providers pane with nothing stored", () => {
    expect(readSettingsSection()).toBe("providers")
  })

  it("round-trips the pane that was open", () => {
    writeSettingsSection("sandbox")

    expect(readSettingsSection()).toBe("sandbox")
  })

  // The opposite of the sort pref, and the reason this one is not parsed
  // against a known set: a provider's section id only exists while that
  // provider is enabled, so an id this build cannot place has to come back
  // whole. The screen resolves it to its first section for as long as the
  // provider is off, and the pane is there again when it is turned back on.
  it("keeps a section id it cannot place, for the screen to resolve", () => {
    writeSettingsSection("provider-crush")

    expect(readSettingsSection()).toBe("provider-crush")
  })
})

describe("the stored search box", () => {
  it("reads empty with nothing stored", () => {
    expect(readSettingsQuery()).toBe("")
  })

  it("round-trips what was typed", () => {
    writeSettingsQuery("sand")

    expect(readSettingsQuery()).toBe("sand")
  })

  // Clearing the box is a state to remember like any other: a screen left with
  // the full nav must not come back filtered by the last search.
  it("round-trips an emptied box", () => {
    writeSettingsQuery("sand")
    writeSettingsQuery("")

    expect(readSettingsQuery()).toBe("")
  })
})

// Both are global on purpose (see the file's docblock): the nav is the same
// list of panes in every project, so nothing on this screen is keyed by one.
describe("the scope of these prefs", () => {
  it("answers with the same section and box whichever project asks", () => {
    writeSettingsSection("hotkeys")
    writeSettingsQuery("hot")

    expect([...stored.keys()].sort()).toEqual(["lich.settings.query", "lich.settings.section"])
  })
})
