import { beforeEach, describe, expect, it, vi } from "vitest"
import { enabledProviders, type ProviderState } from "./providers-store"
import {
  readSettingsProvider,
  readSettingsQuery,
  readSettingsSection,
  writeSettingsProvider,
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

// The screen's own nav, which is what the section is parsed against.
const SECTIONS = ["appearance", "hotkeys", "providers", "sandbox"]

describe("the stored section", () => {
  it("opens on the providers pane with nothing stored", () => {
    expect(readSettingsSection(SECTIONS)).toBe("providers")
  })

  it("round-trips the pane that was open", () => {
    writeSettingsSection("sandbox")

    expect(readSettingsSection(SECTIONS)).toBe("sandbox")
  })

  // The `provider-<id>` panes are still in the storage of anyone who used a
  // build from before Providers became one section, and a pane this one cannot
  // draw must not be where the screen tries to land.
  it("opens the default pane for a section id it cannot place", () => {
    writeSettingsSection("provider-crush")

    expect(readSettingsSection(SECTIONS)).toBe("providers")
  })

  // And the value goes with it: resolving the same dead id at every launch for
  // the life of the install is a landing, not a fix.
  it("rewrites the stored id it could not place", () => {
    writeSettingsSection("provider-crush")
    readSettingsSection(SECTIONS)

    expect(stored.get("lich.settings.section")).toBe("providers")
  })

  it("leaves a section it can place in storage untouched", () => {
    writeSettingsSection("hotkeys")
    readSettingsSection(SECTIONS)

    expect(stored.get("lich.settings.section")).toBe("hotkeys")
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

const provider = (id: string, enabled: boolean): ProviderState => ({
  id: id as ProviderState["id"],
  name: id,
  binary: id,
  installed: true,
  source: "path",
  enabled,
  docs: `https://example.test/${id}`,
})

// What ProvidersPane does with the stored id: the enabled roster decides
// whether the screen it names is still reachable.
const open = (list: ProviderState[]) =>
  enabledProviders(list).find((p) => p.id === readSettingsProvider())

describe("the stored provider screen", () => {
  it("reads as the list with nothing stored", () => {
    expect(readSettingsProvider()).toBe("")
  })

  it("round-trips the provider that was open", () => {
    writeSettingsProvider("codex")

    expect(readSettingsProvider()).toBe("codex")
  })

  // Unlike the section, this one is not parsed on the way out: the roster is
  // the backend's answer, and the id is only meaningful while that provider is
  // enabled. So a disabled provider's screen resolves to the list — the pane's
  // own rule, composed here out of the two pieces it composes — and the stored
  // id is kept, which is what puts the screen back when it is turned on again.
  it("resolves a disabled provider to the list, and back to its screen when it returns", () => {
    writeSettingsProvider("crush")
    const off = [provider("claude", true), provider("crush", false)]

    expect(open(off)).toBeUndefined()
    expect(open([provider("claude", true), provider("crush", true)])?.id).toBe("crush")
    expect(readSettingsProvider()).toBe("crush")
  })

  it("round-trips a step back out to the list", () => {
    writeSettingsProvider("codex")
    writeSettingsProvider("")

    expect(readSettingsProvider()).toBe("")
  })
})

// All three are global on purpose (see the file's docblock): the nav is the same
// list of panes in every project, so nothing on this screen is keyed by one.
describe("the scope of these prefs", () => {
  it("answers with the same section, box and provider whichever project asks", () => {
    writeSettingsSection("hotkeys")
    writeSettingsQuery("hot")
    writeSettingsProvider("claude")

    expect([...stored.keys()].sort()).toEqual([
      "lich.settings.provider",
      "lich.settings.query",
      "lich.settings.section",
    ])
  })
})
