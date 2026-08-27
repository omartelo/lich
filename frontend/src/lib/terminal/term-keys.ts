import {
  matchesCombo,
  matchesRepeatingCombo,
  type HotkeyBinding,
  type Hotkeys,
  type KeyState,
} from "@/lib/hotkeys"
import { isMac, isWindows } from "@/lib/platform"

const ERASE_PREVIOUS_WORD_SEQUENCE = "\x17"
const ATTACH_CLIPBOARD_IMAGE_SEQUENCE = "\x16"
const WINDOWS_ATTACH_CLIPBOARD_IMAGE_SEQUENCE = "\x1bv"
const INSERT_NEWLINE_SEQUENCE = "\x1b\r"

// xterm.js loses the TUI intent behind three configurable bindings, so matched
// presses write substitute sequences directly through
// attachCustomKeyEventHandler:
//
// - Ctrl+Backspace: xterm sends BS (\x08), a single-character erase. ETB
//   (\x17, readline's unix-word-rubout) asks line editors to erase the word.
// - Ctrl+V: Chromium otherwise pastes text and hides the press from TUIs that
//   read the clipboard themselves for image attach. Linux/macOS receive SYN
//   (\x16), like a real terminal. The physical/default binding remains Ctrl+V
//   on Windows too, but Claude Code there expects ESC v, so lich translates the
//   same press to that sequence. Ctrl+Shift+V remains native text paste.
// - Shift+Enter: xterm sends plain CR, indistinguishable from Enter. ESC+CR is
//   what TUIs accept as insert-newline without kitty-protocol negotiation.
//
// xterm correctly encodes everything else, including Alt chords.
export function chordSequence(
  event: KeyState,
  hotkeys: Hotkeys,
  mac = isMac,
  windows = isWindows,
): string | null {
  if (matchesRepeatingCombo(event, hotkeys.eraseTerminalWord, mac)) {
    return ERASE_PREVIOUS_WORD_SEQUENCE
  }
  if (matchesRepeatingCombo(event, hotkeys.attachClipboardImage, mac)) {
    return windows ? WINDOWS_ATTACH_CLIPBOARD_IMAGE_SEQUENCE : ATTACH_CLIPBOARD_IMAGE_SEQUENCE
  }
  if (matchesRepeatingCombo(event, hotkeys.insertTerminalNewline, mac)) {
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
  return !isEditableOutsideTerminal(target) && matchesCombo(event, binding, mac)
}
