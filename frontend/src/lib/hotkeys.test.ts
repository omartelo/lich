import { describe, expect, it } from "vitest"
import {
  comboFromEvent,
  DEFAULT_HOTKEYS,
  formatCombo,
  hotkeyConflicts,
  matchesCombo,
  mergeHotkeys,
  sameCombo,
  type Combo,
  type KeyState,
} from "./hotkeys"

const key = (over: Partial<KeyState>): KeyState => ({
  ctrlKey: false,
  metaKey: false,
  shiftKey: false,
  altKey: false,
  key: "",
  repeat: false,
  ...over,
})

describe("matchesCombo", () => {
  const newSession = DEFAULT_HOTKEYS.newSession // Ctrl+Shift+T

  it("matches Ctrl or Cmd for the primary modifier", () => {
    expect(matchesCombo(key({ ctrlKey: true, shiftKey: true, key: "T" }), newSession)).toBe(true)
    expect(matchesCombo(key({ metaKey: true, shiftKey: true, key: "T" }), newSession)).toBe(true)
  })

  it("folds = into + so a combo recorded as + matches the unshifted key", () => {
    const plus: Combo = { mod: true, shift: false, alt: false, key: "+" }
    expect(matchesCombo(key({ ctrlKey: true, key: "=" }), plus)).toBe(true)
  })

  it("ignores key auto-repeat so a held chord fires once", () => {
    expect(
      matchesCombo(key({ ctrlKey: true, shiftKey: true, key: "T", repeat: true }), newSession),
    ).toBe(false)
  })

  it("rejects when a modifier differs", () => {
    expect(matchesCombo(key({ ctrlKey: true, key: "T" }), newSession)).toBe(false) // no shift
    expect(matchesCombo(key({ shiftKey: true, key: "T" }), newSession)).toBe(false) // no mod
  })

  it("rejects a different key", () => {
    expect(matchesCombo(key({ ctrlKey: true, shiftKey: true, key: "N" }), newSession)).toBe(false)
  })
})

describe("comboFromEvent", () => {
  it("captures modifiers and normalizes the key", () => {
    expect(comboFromEvent(key({ ctrlKey: true, shiftKey: true, key: "T" }))).toEqual({
      mod: true,
      shift: true,
      alt: false,
      key: "t",
    })
  })

  it("returns null for a bare modifier press", () => {
    expect(comboFromEvent(key({ ctrlKey: true, key: "Control" }))).toBeNull()
  })

  it("returns null without a primary modifier or Alt (avoids firing while typing)", () => {
    expect(comboFromEvent(key({ key: "t" }))).toBeNull()
    expect(comboFromEvent(key({ shiftKey: true, key: "T" }))).toBeNull()
  })

  it("accepts Alt-only combos", () => {
    expect(comboFromEvent(key({ altKey: true, key: "n" }))).toEqual({
      mod: false,
      shift: false,
      alt: true,
      key: "n",
    })
  })
})

describe("formatCombo", () => {
  const combo: Combo = { mod: true, shift: true, alt: false, key: "t" }

  it("uses named modifiers joined by + off macOS", () => {
    expect(formatCombo(combo, false)).toBe("Ctrl+Shift+T")
  })

  it("uses symbols with no separator on macOS", () => {
    expect(formatCombo(combo, true)).toBe("⌘⇧T")
  })
})

describe("mergeHotkeys", () => {
  it("layers a valid override over the defaults", () => {
    const override = { newSession: { mod: true, shift: false, alt: true, key: "n" } }
    expect(mergeHotkeys(override).newSession).toEqual(override.newSession)
    expect(mergeHotkeys(override).commandPalette).toEqual(DEFAULT_HOTKEYS.commandPalette)
  })

  it("defaults an action absent from stored settings, keeping the rest", () => {
    // What an install predating a new action has on disk.
    const stored = { newSession: { mod: true, shift: false, alt: true, key: "n" } }
    const merged = mergeHotkeys(stored)
    expect(merged.newSession).toEqual(stored.newSession)
    expect(merged.shortcuts).toEqual(DEFAULT_HOTKEYS.shortcuts)
  })

  it("ignores ids that are no longer actions (the old zoom hotkeys)", () => {
    expect(mergeHotkeys({ zoomIn: { mod: true, shift: false, alt: false, key: "+" } })).toEqual(
      DEFAULT_HOTKEYS,
    )
  })

  it("drops malformed entries and non-objects", () => {
    expect(mergeHotkeys({ newSession: { mod: 1, key: "" } })).toEqual(DEFAULT_HOTKEYS)
    expect(mergeHotkeys(null)).toEqual(DEFAULT_HOTKEYS)
    expect(mergeHotkeys("nope")).toEqual(DEFAULT_HOTKEYS)
  })
})

describe("hotkeyConflicts", () => {
  it("reports nothing when every action holds its own combo", () => {
    expect(hotkeyConflicts(DEFAULT_HOTKEYS)).toEqual({})
  })

  it("names the other action on both sides of a collision", () => {
    const clashing = { ...DEFAULT_HOTKEYS, newSession: DEFAULT_HOTKEYS.commandPalette }
    const conflicts = hotkeyConflicts(clashing)
    expect(conflicts.newSession).toEqual(["commandPalette"])
    expect(conflicts.commandPalette).toEqual(["newSession"])
    expect(conflicts.nextSession).toBeUndefined()
  })

  it("names both others when three actions share a combo", () => {
    const combo: Combo = { mod: true, shift: true, alt: false, key: "j" }
    const conflicts = hotkeyConflicts({
      ...DEFAULT_HOTKEYS,
      newSession: combo,
      nextSession: combo,
      prevSession: combo,
    })
    expect(conflicts.nextSession).toEqual(["newSession", "prevSession"])
  })

  it("treats combos differing only in a modifier as distinct", () => {
    const conflicts = hotkeyConflicts({
      ...DEFAULT_HOTKEYS,
      newSession: { ...DEFAULT_HOTKEYS.commandPalette, alt: true },
    })
    expect(conflicts).toEqual({})
  })
})

describe("sameCombo", () => {
  it("compares every field", () => {
    expect(sameCombo(DEFAULT_HOTKEYS.newSession, DEFAULT_HOTKEYS.newSession)).toBe(true)
    expect(sameCombo(DEFAULT_HOTKEYS.newSession, DEFAULT_HOTKEYS.commandPalette)).toBe(false)
  })
})
