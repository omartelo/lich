import { isMac } from "@/lib/platform"
import { readPref, writePref } from "@/lib/prefs"

// Global keyboard shortcuts are user-configurable and persisted with the rest
// of Settings. `mod` is Ctrl on Windows/Linux and Cmd on macOS; `ctrl` is
// literal Control on every platform.
//
// Zoom is deliberately absent: Chromium binds those accelerators to physical
// keys, so zoom-keys.ts matches event.code instead of a character combo.

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
  | "settings"
  | "pulls"
  | "shortcuts"
  | "terminalSearch"
  | "attachClipboardImage"
  | "insertTerminalNewline"
  | "eraseTerminalWord"

export type HotkeyGroup = "sessions" | "view" | "app" | "terminal"

export const HOTKEY_GROUPS: readonly { id: HotkeyGroup; label: string }[] = [
  { id: "sessions", label: "Sessions" },
  { id: "view", label: "View" },
  { id: "app", label: "App" },
  { id: "terminal", label: "Terminal" },
]

export interface Combo {
  /** Platform primary modifier: Ctrl on Windows/Linux, Cmd on macOS. */
  mod: boolean
  /** Literal Control, including on macOS where it differs from `mod`. */
  ctrl: boolean
  shift: boolean
  alt: boolean
  key: string
}

export type HotkeyBinding = Combo | null

export const UNASSIGNED: HotkeyBinding = null
export const UNASSIGNED_LABEL = "Unassigned"

export interface HotkeyAction {
  id: HotkeyId
  label: string
  group: HotkeyGroup
  combo: Combo
}

const combo = (mod: boolean, shift: boolean, alt: boolean, key: string, ctrl = false): Combo => ({
  mod,
  ctrl,
  shift,
  alt,
  key,
})

// Every global default here is a key taken away from the TUI underneath: the
// chord is caught in the window capture phase and never reaches the PTY. What
// each one costs was measured by pressing it into `cat -v` in a live session:
//
// - Ctrl+letter is a control code the shell and Claude Code already bind
//   (Ctrl+R search, Ctrl+U kill, Ctrl+W erase word) — never take one.
// - Ctrl+Shift+letter reaches the PTY as nothing at all: xterm's control-code
//   mapping requires Shift to be up, so no TUI can bind it and the chord is free.
//   It is the family to reach for, minus the letters Chromium keeps for itself.
// - Ctrl+Shift+arrow arrives as a real sequence (CSI 1;6A…D), so it does cost
//   the TUI something. It is spent only where the direction is the meaning.
// - Ctrl+Alt+arrow is the desktop's workspace switch on Linux and would never
//   arrive; Ctrl+Tab and Ctrl+PageUp/Down are Chromium's own tab accelerators.
//
// Terminal translations are deliberate exceptions: their action is the
// substitute sequence lich writes to the PTY.
//
// HOTKEY_ACTIONS drives the defaults, Settings and the shortcuts overlay.
export const HOTKEY_ACTIONS: readonly HotkeyAction[] = [
  {
    id: "newSession",
    label: "New session",
    group: "sessions",
    combo: combo(true, true, false, "t"),
  },
  // B for branch: the dialog's subject is which branch the checkout is cut
  // from. Ctrl+Shift+W would read better but is Chromium's close-window chord.
  {
    id: "newWorktree",
    label: "New worktree session",
    group: "sessions",
    combo: combo(true, true, false, "b"),
  },
  {
    id: "renameSession",
    label: "Rename the active session",
    group: "sessions",
    combo: combo(true, true, false, "e"),
  },
  // The remaining card actions avoid both spent mnemonics and Chromium's own
  // N, W, Q, I, J, C, O, M, A and R, plus IBus's Ctrl+Shift+U.
  {
    id: "closeSession",
    label: "Close the active session",
    group: "sessions",
    combo: combo(true, true, false, "x"),
  },
  {
    id: "togglePin",
    label: "Pin or unpin the active session",
    group: "sessions",
    combo: combo(true, true, false, "k"),
  },
  {
    id: "openTerminal",
    label: "Open a terminal in the session's directory",
    group: "sessions",
    combo: combo(true, true, false, "l"),
  },
  {
    id: "delegate",
    label: "Delegate to another session",
    group: "sessions",
    combo: combo(true, true, false, "h"),
  },
  // Down/up follows the vertical session list. The project pair below turns the
  // same shape sideways to follow the horizontal project strip.
  {
    id: "nextSession",
    label: "Next session",
    group: "sessions",
    combo: combo(true, true, false, "ArrowDown"),
  },
  {
    id: "prevSession",
    label: "Previous session",
    group: "sessions",
    combo: combo(true, true, false, "ArrowUp"),
  },
  // Ctrl+Shift+Enter reaches the PTY as a plain CR, indistinguishable from
  // Enter, so downstream applications cannot bind it separately.
  {
    id: "focusTerminal",
    label: "Focus the session terminal",
    group: "sessions",
    combo: combo(true, true, false, "Enter"),
  },
  {
    id: "nextProject",
    label: "Next project",
    group: "view",
    combo: combo(true, true, false, "ArrowRight"),
  },
  {
    id: "prevProject",
    label: "Previous project",
    group: "view",
    combo: combo(true, true, false, "ArrowLeft"),
  },
  {
    id: "toggleSidebar",
    label: "Toggle the session sidebar",
    group: "view",
    combo: combo(true, true, false, "s"),
  },
  {
    id: "toggleDock",
    label: "Toggle the right dock",
    group: "view",
    combo: combo(true, true, false, "d"),
  },
  {
    id: "commandPalette",
    label: "Command palette",
    group: "app",
    combo: combo(true, false, false, "k"),
  },
  // Ctrl+, is both the conventional preferences chord and one for which the
  // terminal encodes no control code.
  {
    id: "settings",
    label: "Settings",
    group: "app",
    combo: combo(true, false, false, ","),
  },
  {
    id: "pulls",
    label: "Pull requests",
    group: "app",
    combo: combo(true, true, false, "p"),
  },
  {
    id: "shortcuts",
    label: "Keyboard shortcuts",
    group: "app",
    combo: combo(true, false, false, "/"),
  },
  {
    id: "terminalSearch",
    label: "Search the session's output",
    group: "terminal",
    combo: combo(false, false, false, "f", true),
  },
  {
    id: "attachClipboardImage",
    label: "Attach an image from the clipboard",
    group: "terminal",
    combo: combo(false, false, false, "v", true),
  },
  {
    id: "insertTerminalNewline",
    label: "Insert a newline without sending",
    group: "terminal",
    combo: combo(false, true, false, "Enter"),
  },
  {
    id: "eraseTerminalWord",
    label: "Erase the previous word",
    group: "terminal",
    combo: combo(false, false, false, "Backspace", true),
  },
]

export type Hotkeys = Record<HotkeyId, HotkeyBinding>

export function defaultHotkeys(): Hotkeys {
  return Object.fromEntries(HOTKEY_ACTIONS.map((action) => [action.id, action.combo])) as Hotkeys
}

export const DEFAULT_HOTKEYS: Hotkeys = defaultHotkeys()

export type KeyState = Pick<
  KeyboardEvent,
  "ctrlKey" | "metaKey" | "shiftKey" | "altKey" | "key" | "repeat"
>

const MODIFIER_KEYS: Record<string, true> = {
  Control: true,
  Meta: true,
  Shift: true,
  Alt: true,
  AltGraph: true,
}
const STORAGE_KEY = "lich.hotkeys"
// stopPropagation does not suppress sibling listeners on window; the event
// itself records the first successful lich action without retaining it.
const claimedHotkeyEvents = new WeakSet<object>()

function normalizeKey(key: string): string {
  if (key === "=") return "+"
  return key.length === 1 ? key.toLowerCase() : key
}

function normalizedShift(key: string, shift: boolean): boolean {
  return normalizeKey(key) === "+" ? false : shift
}

function matchesComboWithRepeat(
  event: KeyState,
  binding: HotkeyBinding,
  mac: boolean,
  allowRepeat: boolean,
): boolean {
  if (!binding || (event.repeat && !allowRepeat)) return false
  const ctrl = binding.ctrl || (binding.mod && !mac)
  const meta = binding.mod && mac
  return (
    event.ctrlKey === ctrl &&
    event.metaKey === meta &&
    normalizedShift(event.key, event.shiftKey) === binding.shift &&
    event.altKey === binding.alt &&
    normalizeKey(event.key) === binding.key
  )
}

export function matchesCombo(event: KeyState, binding: HotkeyBinding, mac = isMac): boolean {
  return matchesComboWithRepeat(event, binding, mac, false)
}

export function matchesRepeatingCombo(
  event: KeyState,
  binding: HotkeyBinding,
  mac = isMac,
): boolean {
  return matchesComboWithRepeat(event, binding, mac, true)
}

export function claimHotkey<Result>(
  event: object,
  matched: boolean,
  handler: () => Result,
): boolean {
  if (!matched || claimedHotkeyEvents.has(event)) return false
  if (handler() === false) return false
  claimedHotkeyEvents.add(event)
  return true
}

export function comboFromEvent(event: KeyState, mac = isMac): Combo | null {
  if (MODIFIER_KEYS[event.key] || (event.ctrlKey && event.metaKey)) return null
  const mod = mac ? event.metaKey : event.ctrlKey
  const ctrl = mac && event.ctrlKey
  if (!mod && !ctrl && !event.shiftKey && !event.altKey) return null
  return {
    mod,
    ctrl,
    shift: normalizedShift(event.key, event.shiftKey),
    alt: event.altKey,
    key: normalizeKey(event.key),
  }
}

export function isRecordingTarget(event: Event): boolean {
  const target = event.target as HTMLElement | null
  return !!target?.closest("[data-hotkey-capturing]")
}

export function sameCombo(a: HotkeyBinding, b: HotkeyBinding): boolean {
  if (!a || !b) return a === b
  return (
    a.mod === b.mod &&
    a.ctrl === b.ctrl &&
    a.shift === b.shift &&
    a.alt === b.alt &&
    a.key === b.key
  )
}

function comboKey(binding: Combo, mac: boolean): string {
  const ctrl = binding.ctrl || (binding.mod && !mac)
  const meta = binding.mod && mac
  return `${ctrl}|${meta}|${binding.shift}|${binding.alt}|${binding.key}`
}

export function hotkeyLabel(id: HotkeyId): string {
  return HOTKEY_ACTIONS.find((action) => action.id === id)?.label ?? id
}

export function hotkeyConflicts(
  hotkeys: Hotkeys,
  mac = isMac,
): Partial<Record<HotkeyId, HotkeyId[]>> {
  const byCombo = new Map<string, HotkeyId[]>()
  for (const action of HOTKEY_ACTIONS) {
    const binding = hotkeys[action.id]
    if (!binding) continue
    const key = comboKey(binding, mac)
    const held = byCombo.get(key)
    if (held) held.push(action.id)
    else byCombo.set(key, [action.id])
  }
  const conflicts: Partial<Record<HotkeyId, HotkeyId[]>> = {}
  for (const ids of byCombo.values()) {
    if (ids.length < 2) continue
    for (const id of ids) conflicts[id] = ids.filter((other) => other !== id)
  }
  return conflicts
}

function formatKey(key: string): string {
  if (key === " ") return "Space"
  if (key.startsWith("Arrow")) return key.slice("Arrow".length)
  return key.length === 1 ? key.toUpperCase() : key
}

export function formatCombo(binding: HotkeyBinding, mac: boolean): string {
  if (!binding) return UNASSIGNED_LABEL
  const primarySymbols = mac && binding.mod
  const parts: string[] = []
  if (binding.mod) parts.push(mac ? "⌘" : "Ctrl")
  if (binding.ctrl) parts.push("Ctrl")
  if (binding.shift) parts.push(primarySymbols ? "⇧" : "Shift")
  if (binding.alt) parts.push(primarySymbols ? "⌥" : "Alt")
  parts.push(formatKey(binding.key))
  return parts.join(primarySymbols ? "" : "+")
}

function parsedCombo(value: unknown): Combo | null | undefined {
  if (value === null) return null
  if (!value || typeof value !== "object") return undefined
  const stored = value as Record<string, unknown>
  const legacy = stored.ctrl === undefined
  if (
    typeof stored.mod !== "boolean" ||
    (!legacy && typeof stored.ctrl !== "boolean") ||
    typeof stored.shift !== "boolean" ||
    typeof stored.alt !== "boolean" ||
    typeof stored.key !== "string"
  ) {
    return undefined
  }
  if (stored.key.length === 0) return null
  const key = normalizeKey(stored.key)
  return {
    mod: stored.mod,
    ctrl: legacy ? false : (stored.ctrl as boolean),
    shift: normalizedShift(key, stored.shift),
    alt: stored.alt,
    key,
  }
}

export function mergeHotkeys(overrides: unknown): Hotkeys {
  const result = defaultHotkeys()
  if (!overrides || typeof overrides !== "object") return result
  for (const id of Object.keys(result) as HotkeyId[]) {
    const parsed = parsedCombo((overrides as Record<string, unknown>)[id])
    if (parsed && !parsed.mod && !parsed.ctrl && !parsed.shift && !parsed.alt) {
      continue
    }
    if (parsed !== undefined) result[id] = parsed
  }
  return result
}

export function loadHotkeys(): Hotkeys {
  try {
    const raw = readPref(STORAGE_KEY)
    return raw ? mergeHotkeys(JSON.parse(raw)) : defaultHotkeys()
  } catch {
    return defaultHotkeys()
  }
}

export function saveHotkeys(hotkeys: Hotkeys): void {
  writePref(STORAGE_KEY, JSON.stringify(hotkeys))
}
