import { beforeEach, describe, expect, it, vi } from "vitest"
import {
  comboFromEvent,
  DEFAULT_HOTKEYS,
  defaultHotkeys,
  formatCombo,
  hotkeyConflicts,
  loadHotkeys,
  matchesCombo,
  mergeHotkeys,
  sameCombo,
  saveHotkeys,
  type Combo,
  type HotkeyId,
  type KeyState,
} from "./hotkeys"

const stored = new Map<string, string>()

vi.stubGlobal("localStorage", {
  getItem: (key: string) => stored.get(key) ?? null,
  setItem: (key: string, value: string) => stored.set(key, value),
  removeItem: (key: string) => stored.delete(key),
})

const key = (over: Partial<KeyState>): KeyState => ({
  ctrlKey: false,
  metaKey: false,
  shiftKey: false,
  altKey: false,
  key: "",
  repeat: false,
  ...over,
})

function bound(id: HotkeyId): Combo {
  const binding = DEFAULT_HOTKEYS[id]
  if (!binding) throw new Error(`${id} has no default`)
  return binding
}

describe("matchesCombo", () => {
  it("resolves the primary modifier to Ctrl off macOS and Cmd on macOS", () => {
    const binding = bound("newSession")
    expect(
      matchesCombo(key({ ctrlKey: true, shiftKey: true, key: "T" }), binding, { mac: false }),
    ).toBe(true)
    expect(
      matchesCombo(key({ metaKey: true, shiftKey: true, key: "T" }), binding, { mac: true }),
    ).toBe(true)
    expect(
      matchesCombo(key({ ctrlKey: true, shiftKey: true, key: "T" }), binding, { mac: true }),
    ).toBe(false)
  })

  it("keeps literal Control distinct from Cmd on macOS", () => {
    const binding = bound("terminalSearch")
    expect(matchesCombo(key({ ctrlKey: true, key: "f" }), binding, { mac: true })).toBe(true)
    expect(matchesCombo(key({ metaKey: true, key: "f" }), binding, { mac: true })).toBe(false)
  })

  it("treats the shifted Equal key and + as the same zoom-in chord", () => {
    const binding = bound("zoomIn")
    expect(
      matchesCombo(key({ ctrlKey: true, shiftKey: true, key: "+" }), binding, { mac: false }),
    ).toBe(true)
    expect(matchesCombo(key({ ctrlKey: true, key: "=" }), binding, { mac: false })).toBe(true)
  })

  it("ignores repeat and unassigned bindings", () => {
    expect(
      matchesCombo(
        key({ ctrlKey: true, shiftKey: true, key: "t", repeat: true }),
        bound("newSession"),
      ),
    ).toBe(false)
    expect(matchesCombo(key({ ctrlKey: true, key: "f" }), null)).toBe(false)
  })

  it("allows repeat only when the caller opts in", () => {
    const event = key({ ctrlKey: true, shiftKey: true, key: "t", repeat: true })
    expect(matchesCombo(event, bound("newSession"))).toBe(false)
    expect(matchesCombo(event, bound("newSession"), { allowRepeat: true })).toBe(true)
  })
})

describe("comboFromEvent", () => {
  it("records platform-primary and literal Control separately", () => {
    expect(comboFromEvent(key({ ctrlKey: true, key: "k" }), false)).toEqual({
      mod: true,
      ctrl: false,
      shift: false,
      alt: false,
      key: "k",
    })
    expect(comboFromEvent(key({ ctrlKey: true, key: "f" }), true)).toEqual({
      mod: false,
      ctrl: true,
      shift: false,
      alt: false,
      key: "f",
    })
    expect(comboFromEvent(key({ metaKey: true, key: "k" }), true)?.mod).toBe(true)
  })

  it("rejects bare keys for global actions and permits them for terminal actions", () => {
    expect(comboFromEvent(key({ shiftKey: true, key: "Enter" }))).toBeNull()
    expect(comboFromEvent(key({ shiftKey: true, key: "Enter" }), false, true)).toEqual({
      mod: false,
      ctrl: false,
      shift: true,
      alt: false,
      key: "Enter",
    })
  })

  it("waits through modifier-only keydowns", () => {
    expect(comboFromEvent(key({ ctrlKey: true, key: "Control" }))).toBeNull()
  })
})

describe("formatCombo", () => {
  it("formats primary bindings for the platform", () => {
    expect(formatCombo(bound("newSession"), false)).toBe("Ctrl+Shift+T")
    expect(formatCombo(bound("newSession"), true)).toBe("⌘⇧T")
  })

  it("spells literal Control on macOS and names an empty binding", () => {
    expect(formatCombo(bound("terminalSearch"), true)).toBe("Ctrl+F")
    expect(formatCombo(null, true)).toBe("Unassigned")
  })
})

describe("defaultHotkeys", () => {
  it("uses the provider's platform-specific clipboard image chord", () => {
    expect(defaultHotkeys(false).attachClipboardImage).toEqual({
      mod: false,
      ctrl: true,
      shift: false,
      alt: false,
      key: "v",
    })
    expect(defaultHotkeys(true).attachClipboardImage).toEqual({
      mod: false,
      ctrl: false,
      shift: false,
      alt: true,
      key: "v",
    })
  })
})

describe("mergeHotkeys", () => {
  it("migrates legacy combos and fills every action added since they were saved", () => {
    const merged = mergeHotkeys({
      newSession: { mod: true, shift: false, alt: true, key: "N" },
    })
    expect(merged.newSession).toEqual({
      mod: true,
      ctrl: false,
      shift: false,
      alt: true,
      key: "n",
    })
    expect(merged.zoomIn).toEqual(DEFAULT_HOTKEYS.zoomIn)
    expect(merged.terminalSearch).toEqual(DEFAULT_HOTKEYS.terminalSearch)
  })

  it("does not revive zoom overrides saved before zoom bindings were retired", () => {
    const merged = mergeHotkeys({
      zoomIn: { mod: true, shift: true, alt: false, key: "+" },
      zoomOut: { mod: true, shift: false, alt: true, key: "-" },
    })
    expect(merged.zoomIn).toEqual(DEFAULT_HOTKEYS.zoomIn)
    expect(merged.zoomOut).toEqual(DEFAULT_HOTKEYS.zoomOut)
  })

  it("rejects a persisted terminal search binding without modifiers", () => {
    const merged = mergeHotkeys({
      terminalSearch: { mod: false, ctrl: false, shift: false, alt: false, key: "f" },
    })
    expect(merged.terminalSearch).toEqual(DEFAULT_HOTKEYS.terminalSearch)
  })

  it("preserves explicit unassigned bindings", () => {
    expect(mergeHotkeys({ terminalSearch: null }).terminalSearch).toBeNull()
  })

  it("drops malformed and unknown entries without disturbing defaults", () => {
    const merged = mergeHotkeys({ newSession: { mod: 1, key: "" }, retiredAction: null })
    expect(merged).toEqual(DEFAULT_HOTKEYS)
  })
})

describe("hotkeyConflicts", () => {
  it("reports both sides of a collision", () => {
    const conflicts = hotkeyConflicts({
      ...DEFAULT_HOTKEYS,
      newSession: DEFAULT_HOTKEYS.commandPalette,
    })
    expect(conflicts.newSession).toEqual(["commandPalette"])
    expect(conflicts.commandPalette).toEqual(["newSession"])
  })

  it("detects primary and literal Ctrl as the same chord off macOS only", () => {
    const hotkeys = {
      ...DEFAULT_HOTKEYS,
      newSession: { mod: false, ctrl: true, shift: false, alt: false, key: "k" },
    }
    expect(hotkeyConflicts(hotkeys, false).newSession).toEqual(["commandPalette"])
    expect(hotkeyConflicts(hotkeys, true)).toEqual({})
  })

  it("ignores unassigned actions", () => {
    expect(
      hotkeyConflicts({
        ...DEFAULT_HOTKEYS,
        newSession: null,
        commandPalette: null,
      }),
    ).toEqual({})
  })
})

describe("sameCombo", () => {
  it("compares bindings including the unassigned state", () => {
    expect(sameCombo(null, null)).toBe(true)
    expect(sameCombo(null, bound("newSession"))).toBe(false)
    expect(sameCombo(bound("newSession"), bound("newSession"))).toBe(true)
  })
})

describe("loadHotkeys", () => {
  beforeEach(() => stored.clear())

  it("round-trips assigned and unassigned bindings", () => {
    saveHotkeys({ ...DEFAULT_HOTKEYS, terminalSearch: null })
    expect(loadHotkeys().terminalSearch).toBeNull()
  })

  it("falls back to defaults for corrupt JSON", () => {
    stored.set("lich.hotkeys", '{"newSession":')
    expect(loadHotkeys()).toEqual(DEFAULT_HOTKEYS)
  })
})
