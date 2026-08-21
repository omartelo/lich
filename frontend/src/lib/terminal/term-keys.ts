import { matchesCombo, type HotkeyBinding, type Hotkeys, type KeyState } from "@/lib/hotkeys"
import { isMac, isWindows } from "@/lib/platform"

const ERASE_PREVIOUS_WORD_SEQUENCE = "\x17"
const ATTACH_CLIPBOARD_IMAGE_SEQUENCE = "\x16"
const WINDOWS_ATTACH_CLIPBOARD_IMAGE_SEQUENCE = "\x1bv"
const INSERT_NEWLINE_SEQUENCE = "\x1b\r"

// xterm loses the TUI intent behind Ctrl+Backspace, Ctrl+V and Shift+Enter, so
// these bindings write substitute sequences directly. Claude Code expects
// Alt+V (ESC v) for image attach on Windows instead of Ctrl+V's SYN.
export function chordSequence(
  event: KeyState,
  hotkeys: Hotkeys,
  mac = isMac,
  windows = isWindows,
): string | null {
  const options = { mac, allowRepeat: true }
  if (matchesCombo(event, hotkeys.eraseTerminalWord, options)) {
    return ERASE_PREVIOUS_WORD_SEQUENCE
  }
  if (matchesCombo(event, hotkeys.attachClipboardImage, options)) {
    return windows ? WINDOWS_ATTACH_CLIPBOARD_IMAGE_SEQUENCE : ATTACH_CLIPBOARD_IMAGE_SEQUENCE
  }
  if (matchesCombo(event, hotkeys.insertTerminalNewline, options)) {
    return INSERT_NEWLINE_SEQUENCE
  }
  return null
}

function isEditableOutsideTerminal(target: EventTarget | null): boolean {
  const element = target as HTMLElement | null
  if (!element || element.closest?.(".xterm")) return false
  const tag = element.tagName?.toLowerCase()
  return tag === "input" || tag === "textarea" || element.isContentEditable
}

export function shouldOpenTerminalSearch(
  event: KeyState,
  binding: HotkeyBinding,
  target: EventTarget | null,
  mac = isMac,
): boolean {
  return !isEditableOutsideTerminal(target) && matchesCombo(event, binding, { mac })
}
