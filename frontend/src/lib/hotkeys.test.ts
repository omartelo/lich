import { describe, expect, it } from "vitest"
import {
  comboFromEvent,
  DEFAULT_HOTKEYS,
  formatCombo,
  hotkeyConflicts,
  adoptStoredHotkeys,
  hotkeyLabel,
  matchesCombo,
  mergeHotkeys,
  parseHotkeys,
  sameCombo,
  UNASSIGNED,
  UNASSIGNED_LABEL,
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

  // The point of an unassigned action: nothing fires it, so the chord the user
  // freed reaches the PTY the way it would if lich had never bound it.
  it("never matches an unassigned combo", () => {
    expect(matchesCombo(key({ ctrlKey: true, shiftKey: true, key: "T" }), UNASSIGNED)).toBe(false)
    expect(matchesCombo(key({}), UNASSIGNED)).toBe(false)
    expect(matchesCombo(key({ ctrlKey: true, key: "" }), UNASSIGNED)).toBe(false)
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

  it("names an unassigned combo instead of printing modifiers alone", () => {
    expect(formatCombo(UNASSIGNED, false)).toBe(UNASSIGNED_LABEL)
    expect(formatCombo(UNASSIGNED, true)).toBe(UNASSIGNED_LABEL)
    expect(formatCombo({ mod: true, shift: true, alt: false, key: "" }, false)).toBe(
      UNASSIGNED_LABEL,
    )
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

  it("normalizes a stored key, so an uppercase one still fires", () => {
    // Well formed enough to survive validation, but matchesCombo compares
    // against a normalized event key: kept as "T" the shortcut would be dead.
    const merged = mergeHotkeys({ newSession: { mod: true, shift: true, alt: false, key: "T" } })
    expect(merged.newSession.key).toBe("t")
    expect(matchesCombo(key({ ctrlKey: true, shiftKey: true, key: "T" }), merged.newSession)).toBe(
      true,
    )
  })

  it("ignores ids that are no longer actions (the old zoom hotkeys)", () => {
    expect(mergeHotkeys({ zoomIn: { mod: true, shift: false, alt: false, key: "+" } })).toEqual(
      DEFAULT_HOTKEYS,
    )
  })

  it("keeps an unassigned action unassigned", () => {
    expect(mergeHotkeys({ newSession: UNASSIGNED }).newSession).toEqual(UNASSIGNED)
  })

  it("folds modifiers with no key into the one unassigned shape", () => {
    // What a hand-edited store can hold: nothing to press, but flags set. Two
    // shapes for one nothing would read as a conflict and as a live binding.
    expect(
      mergeHotkeys({ newSession: { mod: true, shift: true, alt: true, key: "" } }).newSession,
    ).toEqual(UNASSIGNED)
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

  // Nothing is not a chord two actions can both hold: an unassigned action is
  // bound to no key, so it collides with nothing, including another one.
  it("never reports an unassigned action as a conflict", () => {
    const conflicts = hotkeyConflicts({
      ...DEFAULT_HOTKEYS,
      newSession: UNASSIGNED,
      nextSession: UNASSIGNED,
    })
    expect(conflicts).toEqual({})
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

  // What the settings row reads to decide whether Reset and Unassign are live.
  it("separates unassigned from a real binding", () => {
    expect(sameCombo(UNASSIGNED, UNASSIGNED)).toBe(true)
    expect(sameCombo(UNASSIGNED, DEFAULT_HOTKEYS.newSession)).toBe(false)
  })
})

describe("parseHotkeys", () => {
  it("answers the defaults for a value nobody has set", () => {
    expect(parseHotkeys("")).toEqual(DEFAULT_HOTKEYS)
  })

  it("round-trips what the provider stores", () => {
    const mine: Combo = { mod: true, shift: true, alt: false, key: "j" }
    const raw = JSON.stringify({ ...DEFAULT_HOTKEYS, newSession: mine })

    expect(parseHotkeys(raw).newSession).toEqual(mine)
  })

  it("round-trips an unassigned action, and takes its default back on reset", () => {
    const cleared = parseHotkeys(JSON.stringify({ ...DEFAULT_HOTKEYS, newSession: UNASSIGNED }))
    expect(cleared.newSession).toEqual(UNASSIGNED)
    expect(matchesCombo(key({ ctrlKey: true, shiftKey: true, key: "T" }), cleared.newSession)).toBe(
      false,
    )

    // What resetHotkey writes back (settings.tsx): the default for that id.
    const reset = { ...cleared, newSession: DEFAULT_HOTKEYS.newSession }
    expect(parseHotkeys(JSON.stringify(reset))).toEqual(DEFAULT_HOTKEYS)
  })

  // A stored setting must never be able to break a launch: the value is a string
  // somebody can hand-edit, and half of one is what an interrupted write leaves.
  it("falls back to the defaults for a value that is not JSON", () => {
    expect(parseHotkeys('{"newSession":')).toEqual(DEFAULT_HOTKEYS)
  })

  it("falls back for JSON that is not an object at all", () => {
    expect(parseHotkeys('"ctrl+shift+t"')).toEqual(DEFAULT_HOTKEYS)
  })

  // Stored under a key the build no longer has, beside one it does: the known
  // override stands and the stranger is dropped.
  it("keeps a valid override and ignores what is not an action", () => {
    const parsed = parseHotkeys(
      JSON.stringify({
        newSession: { mod: true, shift: true, alt: false, key: "J" },
        zoomIn: { mod: true, shift: false, alt: false, key: "+" },
      }),
    )

    expect(parsed.newSession.key).toBe("j")
    expect(parsed).not.toHaveProperty("zoomIn")
  })
})

describe("adoptStoredHotkeys", () => {
  const mine: Combo = { mod: true, shift: true, alt: false, key: "j" }
  const raw = (combo: Combo) => JSON.stringify({ ...DEFAULT_HOTKEYS, newSession: combo })

  it("reads the workspace copy, and asks for no migration", () => {
    const { hotkeys, migrate } = adoptStoredHotkeys(raw(mine), null)

    expect(hotkeys.newSession).toEqual(mine)
    expect(migrate).toBe(false)
  })

  // The install that predates the move: the page's entry is the only record, so
  // it is adopted and handed to the caller to write back once.
  it("adopts the page copy and asks to migrate it", () => {
    const { hotkeys, migrate } = adoptStoredHotkeys("", raw(mine))

    expect(hotkeys.newSession).toEqual(mine)
    expect(migrate).toBe(true)
  })

  // Both stores answering is the launch after the migration wrote but before the
  // page entry was dropped, and a stale page copy must never win.
  it("lets the workspace copy beat the page copy", () => {
    const theirs: Combo = { mod: true, shift: true, alt: false, key: "y" }
    const { hotkeys, migrate } = adoptStoredHotkeys(raw(theirs), raw(mine))

    expect(hotkeys.newSession).toEqual(theirs)
    expect(migrate).toBe(false)
  })

  it("answers the defaults, and no migration, for an install with neither", () => {
    const { hotkeys, migrate } = adoptStoredHotkeys("", null)

    expect(hotkeys).toEqual(DEFAULT_HOTKEYS)
    expect(migrate).toBe(false)
  })

  // A page entry nobody ever rebound still migrates: writing the defaults is
  // what makes the database the record, and re-reading the entry after this
  // would take the migration for one still pending.
  it("migrates an empty page entry rather than leaving it behind", () => {
    expect(adoptStoredHotkeys("", "{}")).toEqual({ hotkeys: DEFAULT_HOTKEYS, migrate: true })
  })
})

describe("hotkeyLabel", () => {
  it("names an action", () => {
    expect(hotkeyLabel("commandPalette")).toBe("Command palette")
  })

  // Nothing in the app can ask for an id that is not an action, but the label is
  // what a settings row renders — an empty one would be a blank line.
  it("falls back to the id for an action this build does not have", () => {
    expect(hotkeyLabel("zoomIn" as never)).toBe("zoomIn")
  })
})
