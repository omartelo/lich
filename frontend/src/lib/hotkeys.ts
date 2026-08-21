import { isMac, isWindows } from "@/lib/platform"
import { readPref, writePref } from "@/lib/prefs"

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
  | "zoomIn"
  | "zoomOut"
  | "zoomReset"
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

export interface HotkeyAction {
  id: HotkeyId
  label: string
  group: HotkeyGroup
  combo: Combo
  allowUnmodified?: boolean
}

const combo = (mod: boolean, shift: boolean, alt: boolean, key: string, ctrl = false): Combo => ({
  mod,
  ctrl,
  shift,
  alt,
  key,
})

const imageCombo = (windows: boolean): Combo =>
  windows ? combo(false, false, true, "v") : combo(false, false, false, "v", true)

// Defaults avoid Ctrl+letter where a shifted chord is available: every bound
// global combo is captured before xterm and therefore belongs to lich, not the
// TUI. Terminal translations are deliberate exceptions because their action is
// the sequence lich writes to the PTY.
export const HOTKEY_ACTIONS: readonly HotkeyAction[] = [
  {
    id: "newSession",
    label: "New session",
    group: "sessions",
    combo: combo(true, true, false, "t"),
  },
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
    id: "zoomIn",
    label: "Zoom in",
    group: "view",
    combo: combo(true, false, false, "+"),
  },
  {
    id: "zoomOut",
    label: "Zoom out",
    group: "view",
    combo: combo(true, false, false, "-"),
  },
  {
    id: "zoomReset",
    label: "Reset zoom",
    group: "view",
    combo: combo(true, false, false, "0"),
  },
  {
    id: "commandPalette",
    label: "Command palette",
    group: "app",
    combo: combo(true, false, false, "k"),
  },
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
    allowUnmodified: true,
  },
  {
    id: "attachClipboardImage",
    label: "Attach an image from the clipboard",
    group: "terminal",
    combo: imageCombo(isWindows),
    allowUnmodified: true,
  },
  {
    id: "insertTerminalNewline",
    label: "Insert a newline without sending",
    group: "terminal",
    combo: combo(false, true, false, "Enter"),
    allowUnmodified: true,
  },
  {
    id: "eraseTerminalWord",
    label: "Erase the previous word",
    group: "terminal",
    combo: combo(false, false, false, "Backspace", true),
    allowUnmodified: true,
  },
]

export type Hotkeys = Record<HotkeyId, HotkeyBinding>

export function defaultHotkeys(windows = isWindows): Hotkeys {
  const defaults = Object.fromEntries(
    HOTKEY_ACTIONS.map((action) => [action.id, action.combo]),
  ) as Hotkeys
  defaults.attachClipboardImage = imageCombo(windows)
  return defaults
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

function normalizeKey(key: string): string {
  if (key === "=") return "+"
  return key.length === 1 ? key.toLowerCase() : key
}

function normalizedShift(key: string, shift: boolean): boolean {
  return normalizeKey(key) === "+" ? false : shift
}

export function matchesCombo(event: KeyState, binding: HotkeyBinding, mac = isMac): boolean {
  if (!binding || event.repeat) return false
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

export function comboFromEvent(
  event: KeyState,
  mac = isMac,
  allowUnmodified = false,
): Combo | null {
  if (MODIFIER_KEYS[event.key] || (event.ctrlKey && event.metaKey)) return null
  const mod = mac ? event.metaKey : event.ctrlKey
  const ctrl = mac && event.ctrlKey
  if (!mod && !ctrl && !event.altKey && !allowUnmodified) return null
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
  if (!binding) return "Unassigned"
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
    typeof stored.key !== "string" ||
    stored.key.length === 0
  ) {
    return undefined
  }
  const key = normalizeKey(stored.key)
  return {
    mod: stored.mod,
    ctrl: legacy ? false : (stored.ctrl as boolean),
    shift: normalizedShift(key, stored.shift),
    alt: stored.alt,
    key,
  }
}

function isRetiredZoomOverride(id: HotkeyId, value: unknown): boolean {
  const zoom = id === "zoomIn" || id === "zoomOut" || id === "zoomReset"
  return zoom && !!value && typeof value === "object" && !("ctrl" in value)
}

export function mergeHotkeys(overrides: unknown): Hotkeys {
  const result = defaultHotkeys()
  if (!overrides || typeof overrides !== "object") return result
  for (const id of Object.keys(result) as HotkeyId[]) {
    const value = (overrides as Record<string, unknown>)[id]
    // Zoom bindings were removed because their old shape could not represent
    // Ctrl+Plus correctly. Do not silently revive those retired overrides.
    if (isRetiredZoomOverride(id, value)) continue
    const parsed = parsedCombo(value)
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
