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

export const TERMINAL_TITLE = "Terminal"
export const PASSTHROUGH_TITLE = "Passed through to the agent"

// lich's own chord, and not a rebindable one: it shadows Chromium's Find in
// --app mode, and an accelerator answers to a fixed chord (TerminalView). Ctrl
// on macOS too, for the same reason as the rows below.
export const terminalRows: ShortcutRow[] = [
  { label: "Search the session's output", keys: "Ctrl+F" },
]

// These are spelled out rather than built from a Combo: they are matched on
// event.ctrlKey (term-keys.ts), so the modifier is Control on macOS too and
// formatCombo's `mod` — which prints ⌘ there — would lie.
export function passthroughRows(isWindows: boolean, isMac = false): ShortcutRow[] {
  return [
    // Claude Code reads the clipboard itself on this one; Windows binds it to
    // Alt+V, so the chord lich sends differs by platform (term-keys.ts). A paste
    // of an image with no text sends the same chord, which is how ⌘V gets there.
    { label: "Attach an image from the clipboard", keys: imageAttachKeys(isWindows, isMac) },
    { label: "Insert a newline without sending", keys: "Shift+Enter" },
    { label: "Erase the previous word", keys: "Ctrl+Backspace" },
  ]
}

function imageAttachKeys(isWindows: boolean, isMac: boolean): string {
  if (isWindows) {
    return "Alt+V"
  }
  return isMac ? "⌘V or Ctrl+V" : "Ctrl+V"
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
  return [
    ...bound,
    { title: TERMINAL_TITLE, rows: terminalRows },
    { title: PASSTHROUGH_TITLE, rows: passthroughRows(isWindows, isMac) },
  ]
}
