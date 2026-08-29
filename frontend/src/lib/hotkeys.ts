import { readPref, writePref } from "@/lib/prefs"

// Global keyboard shortcuts. Combos are user-configurable and persisted to
// localStorage, matching every other setting (see settings.tsx). `mod` is the
// platform primary modifier — Ctrl on Windows/Linux, Cmd on macOS — so a single
// stored combo works on both.

// Zoom is deliberately absent: those chords shadow Chromium's own accelerators,
// which are bound to physical keys, so they are matched on event.code in
// zoom-keys.ts instead of being character combos a user can rebind.
export type HotkeyId =
  | "commandPalette"
  | "newSession"
  | "newWorktree"
  | "renameSession"
  | "closeSession"
  | "togglePin"
  | "openTerminal"
  | "delegate"
  | "nextSession"
  | "prevSession"
  | "focusTerminal"
  | "nextProject"
  | "prevProject"
  | "toggleSidebar"
  | "toggleDock"
  | "splitBeside"
  | "otherPane"
  | "settings"
  | "pulls"
  | "shortcuts"

/** Which part of the app an action acts on — the grouping both UIs read. */
export type HotkeyGroup = "sessions" | "view" | "app"

export const HOTKEY_GROUPS: readonly { id: HotkeyGroup; label: string }[] = [
  { id: "sessions", label: "Sessions" },
  { id: "view", label: "View" },
  { id: "app", label: "App" },
]

export interface Combo {
  mod: boolean
  shift: boolean
  alt: boolean
  key: string
}

// The stored value of an action bound to nothing: the empty key is what makes
// it unassigned, and no keypress can produce one, so it never matches. It is
// how a user gives a chord back to the TUI in the terminal, which never sees a
// chord the window claims for itself.
export const UNASSIGNED: Combo = { mod: false, shift: false, alt: false, key: "" }

export const UNASSIGNED_LABEL = "Unassigned"

export interface HotkeyAction {
  id: HotkeyId
  label: string
  group: HotkeyGroup
  combo: Combo
}

// Every default here is a key taken away from the TUI underneath: the chord is
// caught in the window capture phase and never reaches the PTY. What each one
// costs was measured by pressing it into `cat -v` in a live session:
//
// - Ctrl+letter is a control code the shell and Claude Code already bind
//   (Ctrl+R search, Ctrl+U kill, Ctrl+W erase word) — never take one.
// - Ctrl+Shift+letter reaches the PTY as *nothing at all*: xterm's control-code
//   mapping requires Shift to be up, so no TUI can bind it and the chord is free.
//   It is the family to reach for, minus the letters Chromium keeps for itself.
// - Ctrl+Shift+arrow arrives as a real sequence (CSI 1;6A…D), so it does cost
//   the TUI something. It is spent only where the direction *is* the meaning.
// - Ctrl+Alt+arrow is the desktop's workspace switch on Linux and would never
//   arrive; Ctrl+Tab and Ctrl+PageUp/Down are Chromium's own tab accelerators.
//
// HOTKEY_ACTIONS drives the defaults, the settings list and the shortcuts
// overlay, in this order.
export const HOTKEY_ACTIONS: readonly HotkeyAction[] = [
  {
    id: "newSession",
    label: "New session",
    group: "sessions",
    combo: { mod: true, shift: true, alt: false, key: "t" },
  },
  // B for branch: the dialog's whole subject is which branch the checkout is
  // cut from. Ctrl+Shift+W would have read better and is Chromium's close
  // window, which a page cannot take back.
  {
    id: "newWorktree",
    label: "New worktree session",
    group: "sessions",
    combo: { mod: true, shift: true, alt: false, key: "b" },
  },
  {
    id: "renameSession",
    label: "Rename the active session",
    group: "sessions",
    combo: { mod: true, shift: true, alt: false, key: "e" },
  },
  // The rest of the card's own actions. Only X carries a mnemonic anyone would
  // guess; the letters that would (P for pin, T for terminal, D for delegate)
  // are spent, and what was left had to avoid Chromium's own (N, W, Q, I, J, C,
  // O, M, A, R) and Ctrl+Shift+U, which starts Unicode entry under IBus.
  {
    id: "closeSession",
    label: "Close the active session",
    group: "sessions",
    combo: { mod: true, shift: true, alt: false, key: "x" },
  },
  {
    id: "togglePin",
    label: "Pin or unpin the active session",
    group: "sessions",
    combo: { mod: true, shift: true, alt: false, key: "k" },
  },
  {
    id: "openTerminal",
    label: "Open a terminal in the session's directory",
    group: "sessions",
    combo: { mod: true, shift: true, alt: false, key: "l" },
  },
  {
    id: "delegate",
    label: "Delegate to another session",
    group: "sessions",
    combo: { mod: true, shift: true, alt: false, key: "h" },
  },
  // Down/up because the sidebar list is vertical; the project pair below is the
  // same shape turned sideways, because the tab strip is horizontal.
  {
    id: "nextSession",
    label: "Next session",
    group: "sessions",
    combo: { mod: true, shift: true, alt: false, key: "ArrowDown" },
  },
  {
    id: "prevSession",
    label: "Previous session",
    group: "sessions",
    combo: { mod: true, shift: true, alt: false, key: "ArrowUp" },
  },
  // The cheapest chord in the app to take: Ctrl+Shift+Enter reaches the PTY as a
  // plain CR, indistinguishable from Enter, so nothing downstream can bind it
  // apart from Enter itself — which the user still has.
  {
    id: "focusTerminal",
    label: "Focus the session terminal",
    group: "sessions",
    combo: { mod: true, shift: true, alt: false, key: "Enter" },
  },
  {
    id: "nextProject",
    label: "Next project",
    group: "view",
    combo: { mod: true, shift: true, alt: false, key: "ArrowRight" },
  },
  {
    id: "prevProject",
    label: "Previous project",
    group: "view",
    combo: { mod: true, shift: true, alt: false, key: "ArrowLeft" },
  },
  {
    id: "toggleSidebar",
    label: "Toggle the session sidebar",
    group: "view",
    combo: { mod: true, shift: true, alt: false, key: "s" },
  },
  {
    id: "toggleDock",
    label: "Toggle the right dock",
    group: "view",
    combo: { mod: true, shift: true, alt: false, key: "d" },
  },
  // O for "open beside", G for "go across". Both are letters on purpose: the
  // family note above is measured for Ctrl+Shift+*letter*, and the chord this
  // pair wants to spell — Ctrl+Shift+\, the editors' split — is not a letter, so
  // whether xterm's mapping lets it through is a question nobody here has put to
  // a `cat -v`. Two free letters that are certainly silent beat one that reads
  // better and might arrive. J is the one to keep off: Chromium answers it with
  // DevTools, which the --app window does open.
  {
    id: "splitBeside",
    label: "Show a second session beside this one",
    group: "view",
    combo: { mod: true, shift: true, alt: false, key: "o" },
  },
  {
    id: "otherPane",
    label: "Focus the other pane",
    group: "view",
    combo: { mod: true, shift: true, alt: false, key: "g" },
  },
  {
    id: "commandPalette",
    label: "Command palette",
    group: "app",
    combo: { mod: true, shift: false, alt: false, key: "k" },
  },
  // Ctrl+, is what every other app opens preferences with, and the terminal
  // encodes no control code for it — a rare case where the familiar chord is
  // also the free one.
  {
    id: "settings",
    label: "Settings",
    group: "app",
    combo: { mod: true, shift: false, alt: false, key: "," },
  },
  {
    id: "pulls",
    label: "Pull requests",
    group: "app",
    combo: { mod: true, shift: true, alt: false, key: "p" },
  },
  {
    id: "shortcuts",
    label: "Keyboard shortcuts",
    group: "app",
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
  if (!combo.key) return false
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

// A combo's identity as a map key, so many can be bucketed in one pass where
// sameCombo only compares two. The separator cannot collide: a key is either a
// single character or a name like "ArrowDown", and neither contains "|".
function comboKey(combo: Combo): string {
  return `${combo.mod}|${combo.shift}|${combo.alt}|${combo.key}`
}

export function hotkeyLabel(id: HotkeyId): string {
  return HOTKEY_ACTIONS.find((action) => action.id === id)?.label ?? id
}

// hotkeyConflicts reports, for every action sharing its combo with another, the
// other actions holding it — in HOTKEY_ACTIONS order. Recording a taken combo is
// allowed (lich does not veto the user's choice), but with two listeners on one
// chord whichever runs first wins and the other action silently stops working,
// so both rows say so.
export function hotkeyConflicts(hotkeys: Hotkeys): Partial<Record<HotkeyId, HotkeyId[]>> {
  const byCombo = new Map<string, HotkeyId[]>()
  for (const action of HOTKEY_ACTIONS) {
    const combo = hotkeys[action.id]
    if (!combo.key) continue
    const key = comboKey(combo)
    const held = byCombo.get(key)
    if (held) {
      held.push(action.id)
    } else {
      byCombo.set(key, [action.id])
    }
  }
  const conflicts: Partial<Record<HotkeyId, HotkeyId[]>> = {}
  for (const ids of byCombo.values()) {
    if (ids.length < 2) continue
    for (const id of ids) {
      conflicts[id] = ids.filter((other) => other !== id)
    }
  }
  return conflicts
}

function formatKey(key: string): string {
  if (key === " ") return "Space"
  if (key.startsWith("Arrow")) return key.slice("Arrow".length)
  return key.length === 1 ? key.toUpperCase() : key
}

export function formatCombo(combo: Combo, isMac: boolean): string {
  if (!combo.key) return UNASSIGNED_LABEL
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
    typeof c.key === "string"
  )
}

// mergeHotkeys layers validated overrides over the defaults, dropping anything
// malformed. Keeps unknown/corrupt persisted data from breaking shortcuts.
//
// The stored key is normalized on the way in, not only on the way out of
// comboFromEvent: a combo hand-edited into localStorage as "T" is well-formed,
// so it survives validation, but matchesCombo compares against a normalized
// event key and would never fire it — a shortcut dead with nothing on screen
// saying why.
export function mergeHotkeys(overrides: unknown): Hotkeys {
  const result: Hotkeys = { ...DEFAULT_HOTKEYS }
  if (overrides && typeof overrides === "object") {
    for (const id of Object.keys(DEFAULT_HOTKEYS) as HotkeyId[]) {
      const value = (overrides as Record<string, unknown>)[id]
      if (!isCombo(value)) continue
      // Modifiers with no key are the same nothing as no key at all; folding
      // them into UNASSIGNED keeps a single stored shape for "bound to nothing",
      // which is what sameCombo and the conflict buckets compare against.
      result[id] = value.key ? { ...value, key: normalizeKey(value.key) } : UNASSIGNED
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
