import { formatCombo, HOTKEY_ACTIONS, HOTKEY_GROUPS, type Hotkeys } from "@/lib/hotkeys"

// The rows the shortcuts overlay lists, and the read-only half of the Hotkeys
// settings pane. Two sources: the rebindable actions, read from the user's
// current bindings, and the chords lich rewrites on the way to the PTY.

export interface ShortcutRow {
  label: string
  keys: string
}

export interface ShortcutGroup {
  title: string
  rows: ShortcutRow[]
}

export const PASSTHROUGH_TITLE = "Passed through to the agent"

// These are spelled out rather than built from a Combo: they are matched on
// event.ctrlKey (term-keys.ts), so the modifier is Control on macOS too and
// formatCombo's `mod` — which prints ⌘ there — would lie.
export function passthroughRows(isWindows: boolean): ShortcutRow[] {
  return [
    // Claude Code reads the clipboard itself on this one; Windows binds it to
    // Alt+V, so the chord lich sends differs by platform (term-keys.ts).
    { label: "Attach an image from the clipboard", keys: isWindows ? "Alt+V" : "Ctrl+V" },
    { label: "Insert a newline without sending", keys: "Shift+Enter" },
    { label: "Erase the previous word", keys: "Ctrl+Backspace" },
  ]
}

export function shortcutGroups(
  hotkeys: Hotkeys,
  isMac: boolean,
  isWindows: boolean,
): ShortcutGroup[] {
  const bound = HOTKEY_GROUPS.map((group) => ({
    title: group.label,
    rows: HOTKEY_ACTIONS.filter((action) => action.group === group.id).map((action) => ({
      label: action.label,
      keys: formatCombo(hotkeys[action.id], isMac),
    })),
  }))
  return [...bound, { title: PASSTHROUGH_TITLE, rows: passthroughRows(isWindows) }]
}
