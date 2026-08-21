import { matchesCombo, type HotkeyBinding, type Hotkeys, type KeyState } from "@/lib/hotkeys"
import { isMac, isWindows } from "@/lib/platform"

export type TermKeyState = KeyState

export function isSearchOpenChord(
  event: TermKeyState,
  binding: HotkeyBinding,
  mac = isMac,
): boolean {
  return matchesCombo(event, binding, mac)
}

export function chordSequence(
  event: TermKeyState,
  hotkeys: Hotkeys,
  mac = isMac,
  windows = isWindows,
): string | null {
  if (matchesCombo(event, hotkeys.eraseTerminalWord, mac)) return "\x17"
  if (matchesCombo(event, hotkeys.attachClipboardImage, mac)) {
    return windows ? "\x1bv" : "\x16"
  }
  if (matchesCombo(event, hotkeys.insertTerminalNewline, mac)) return "\x1b\r"
  return null
}
