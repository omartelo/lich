import { readPref, writePref } from "@/lib/prefs"

// Global keyboard shortcuts. Combos are user-configurable and persisted to
// localStorage, matching every other setting (see settings.tsx). `mod` is the
// platform primary modifier — Ctrl on Windows/Linux, Cmd on macOS — so a single
// stored combo works on both.

// Zoom is deliberately absent: those chords shadow Chromium's own accelerators,
// which are bound to physical keys, so they are matched on event.code in
// zoom-keys.ts instead of being character combos a user can rebind.
export type HotkeyId = "commandPalette" | "newSession" | "nextSession" | "prevSession" | "shortcuts"

export interface Combo {
  mod: boolean
  shift: boolean
  alt: boolean
  key: string
}

export interface HotkeyAction {
  id: HotkeyId
  label: string
  combo: Combo
}

// HOTKEY_ACTIONS drives the defaults and the settings UI list.
export const HOTKEY_ACTIONS: readonly HotkeyAction[] = [
  {
    id: "commandPalette",
    label: "Command palette",
    combo: { mod: true, shift: false, alt: false, key: "k" },
  },
  {
    id: "newSession",
    label: "New session",
    combo: { mod: true, shift: true, alt: false, key: "t" },
  },
  // Down/up because the sidebar list is vertical. Two modifiers plus an arrow is
  // the one shape nothing downstream wants: any Ctrl+letter is a control code a
  // shell or Claude Code already binds, Chromium's own switching accelerators are
  // Ctrl+Tab and Ctrl+PageUp/Down, and Ctrl+Alt+arrow is the desktop's
  // workspace switch on Linux — it would never reach the page.
  {
    id: "nextSession",
    label: "Next session",
    combo: { mod: true, shift: true, alt: false, key: "ArrowDown" },
  },
  {
    id: "prevSession",
    label: "Previous session",
    combo: { mod: true, shift: true, alt: false, key: "ArrowUp" },
  },
  {
    id: "shortcuts",
    label: "Keyboard shortcuts",
    combo: { mod: true, shift: false, alt: false, key: "/" },
  },
]

export type Hotkeys = Record<HotkeyId, Combo>

export const DEFAULT_HOTKEYS: Hotkeys = Object.fromEntries(
  HOTKEY_ACTIONS.map((action) => [action.id, action.combo]),
) as Hotkeys

// The subset of KeyboardEvent the matcher needs — lets tests pass plain objects.
export type KeyState = Pick<
  KeyboardEvent,
  "ctrlKey" | "metaKey" | "shiftKey" | "altKey" | "key" | "repeat"
>

const MODIFIER_KEYS = new Set(["Control", "Meta", "Shift", "Alt", "AltGraph"])
const STORAGE_KEY = "lich.hotkeys"

// normalizeKey folds "=" into "+" (same physical key) and lowercases single
// characters so casing from Shift does not change the identity of the combo.
// Folding a character pair like this is a patch over event.key being layout- and
// Shift-dependent; it is kept because combos recorded before are persisted with
// "+", but the real answer for a physical key is event.code (see zoom-keys.ts).
function normalizeKey(key: string): string {
  if (key === "=") return "+"
  return key.length === 1 ? key.toLowerCase() : key
}

export function matchesCombo(event: KeyState, combo: Combo): boolean {
  // A held chord auto-repeats; every action here is a discrete command (spawn
  // a session, toggle the palette), so only the initial press may fire.
  if (event.repeat) return false
  const mod = event.ctrlKey || event.metaKey
  return (
    mod === combo.mod &&
    event.shiftKey === combo.shift &&
    event.altKey === combo.alt &&
    normalizeKey(event.key) === combo.key
  )
}

// comboFromEvent builds a combo from a captured keypress, or null when the key
// is only a modifier (wait for the real key) or has no primary modifier — a
// bare "t" would fire while typing.
export function comboFromEvent(event: KeyState): Combo | null {
  if (MODIFIER_KEYS.has(event.key)) return null
  const mod = event.ctrlKey || event.metaKey
  if (!mod && !event.altKey) return null
  return {
    mod,
    shift: event.shiftKey,
    alt: event.altKey,
    key: normalizeKey(event.key),
  }
}

// isRecordingTarget reports whether an event originates from a hotkey capture
// field. Global shortcuts bail on it so pressing a combo while rebinding records
// it instead of firing the action.
export function isRecordingTarget(event: Event): boolean {
  const target = event.target as HTMLElement | null
  return !!target?.closest("[data-hotkey-capturing]")
}

export function sameCombo(a: Combo, b: Combo): boolean {
  return a.mod === b.mod && a.shift === b.shift && a.alt === b.alt && a.key === b.key
}

function formatKey(key: string): string {
  if (key === " ") return "Space"
  if (key.startsWith("Arrow")) return key.slice("Arrow".length)
  return key.length === 1 ? key.toUpperCase() : key
}

export function formatCombo(combo: Combo, isMac: boolean): string {
  const parts: string[] = []
  if (combo.mod) parts.push(isMac ? "⌘" : "Ctrl")
  if (combo.shift) parts.push(isMac ? "⇧" : "Shift")
  if (combo.alt) parts.push(isMac ? "⌥" : "Alt")
  parts.push(formatKey(combo.key))
  return parts.join(isMac ? "" : "+")
}

function isCombo(value: unknown): value is Combo {
  if (!value || typeof value !== "object") return false
  const c = value as Record<string, unknown>
  return (
    typeof c.mod === "boolean" &&
    typeof c.shift === "boolean" &&
    typeof c.alt === "boolean" &&
    typeof c.key === "string" &&
    c.key.length > 0
  )
}

// mergeHotkeys layers validated overrides over the defaults, dropping anything
// malformed. Keeps unknown/corrupt persisted data from breaking shortcuts.
export function mergeHotkeys(overrides: unknown): Hotkeys {
  const result: Hotkeys = { ...DEFAULT_HOTKEYS }
  if (overrides && typeof overrides === "object") {
    for (const id of Object.keys(DEFAULT_HOTKEYS) as HotkeyId[]) {
      const value = (overrides as Record<string, unknown>)[id]
      if (isCombo(value)) result[id] = value
    }
  }
  return result
}

export function loadHotkeys(): Hotkeys {
  try {
    const raw = readPref(STORAGE_KEY)
    return raw ? mergeHotkeys(JSON.parse(raw)) : DEFAULT_HOTKEYS
  } catch {
    return DEFAULT_HOTKEYS
  }
}

export function saveHotkeys(hotkeys: Hotkeys): void {
  writePref(STORAGE_KEY, JSON.stringify(hotkeys))
}
