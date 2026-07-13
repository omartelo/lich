// ghostty-web 0.4.0 drops two families of key sequences: Shift+Tab is
// hardcoded to plain "\t" (never CSI Z / backtab) and the key encoder never
// enables ALT_ESC_PREFIX, so Alt+<char> loses its ESC prefix and arrives as
// the bare character. missingKeySequence supplies what the terminal fails to
// produce; TerminalView writes it straight to the PTY via
// attachCustomKeyEventHandler. Delete this once ghostty-web fixes both.

// The subset of KeyboardEvent the matcher needs — lets tests pass plain
// objects. getModifierState and code are optional so fabricated events can
// omit them.
export type TermKeyState = Pick<
  KeyboardEvent,
  "ctrlKey" | "metaKey" | "shiftKey" | "altKey" | "key"
> & { code?: string; getModifierState?: (key: string) => boolean }

export function missingKeySequence(event: TermKeyState): string | null {
  // ghostty-web encodes Ctrl+Backspace as a plain backspace; send ETB (Ctrl+W,
  // readline's unix-word-rubout) so line editors erase the whole word.
  if (
    event.ctrlKey &&
    !event.metaKey &&
    !event.altKey &&
    !event.shiftKey &&
    event.key === "Backspace"
  )
    return "\x17"
  // ghostty-web swallows Ctrl+V so the browser's text-only paste event fires,
  // which hides the keypress from TUIs that read the clipboard themselves on
  // ^V (Claude Code image attach). Deliver SYN like a real terminal; text
  // paste moves to Ctrl+Shift+V (isTextPasteChord).
  if (
    event.ctrlKey &&
    !event.metaKey &&
    !event.altKey &&
    !event.shiftKey &&
    (event.code === "KeyV" || event.key.toLowerCase() === "v")
  )
    return "\x16"
  if (event.ctrlKey || event.metaKey) return null
  // AltGr (ISO Level3) also reports altKey on some layouts; it is composing a
  // character, not an Alt chord — let the terminal handle it.
  if (event.getModifierState?.("AltGraph")) return null
  // WebKitGTK reports Shift+Tab with the GTK keysym name "ISO_Left_Tab" in
  // event.key; the physical event.code stays "Tab". Match both.
  const isTab = event.key === "Tab" || event.code === "Tab"
  if (isTab && event.shiftKey && !event.altKey) return "\x1b[Z"
  // ghostty-web also encodes Shift+Enter as plain "\r", indistinguishable from
  // Enter. ESC+CR is what TUIs (Claude Code, etc.) accept as "insert newline"
  // without needing kitty-protocol negotiation.
  if (event.key === "Enter" && event.shiftKey && !event.altKey) return "\x1b\r"
  if (event.altKey) {
    if (event.key === "Backspace") return "\x1b\x7f"
    if (event.key.length === 1) return "\x1b" + event.key
  }
  return null
}

// SGR mouse wheel report (DECSET 1006). Apps that turn on mouse tracking —
// Claude Code, htop, vim with `mouse=a` — scroll on these by their own line
// increment, which reads as smooth instead of a page jump. ghostty-web 0.4.0
// reports no mouse events at all, so its alt-screen emulation turns the wheel
// into arrow keys; forwarding this is what Claude Code actually asked for (and
// why it warns about arrow keys). Button 64 = wheel up, 65 = down; press-only,
// no release. col/row are the 1-based cell under the pointer. deltaY 0 (a pure
// horizontal wheel) sends nothing.
export function sgrWheelSequence(deltaY: number, col: number, row: number): string | null {
  if (deltaY === 0) return null
  const button = deltaY > 0 ? 65 : 64
  return `\x1b[<${button};${col};${row}M`
}

// Fallback for TUIs without mouse tracking: in the alternate screen ghostty-web
// still emulates the wheel as one arrow key per tick, which Claude Code rejects
// as "arrow keys · use PgUp/PgDn to scroll". Page keys are what those TUIs
// scroll their history with. deltaY > 0 → PgDn; upward → PgUp; 0 → nothing.
export function altScreenWheelSequence(deltaY: number): string | null {
  if (deltaY === 0) return null
  return deltaY > 0 ? "\x1b[6~" : "\x1b[5~"
}

/**
 * Ctrl+Shift+V — the terminal-convention text-paste chord. Handled outside
 * missingKeySequence because pasting needs the async Wails clipboard read,
 * not a fixed byte sequence.
 */
export function isTextPasteChord(event: TermKeyState): boolean {
  return (
    event.ctrlKey &&
    event.shiftKey &&
    !event.metaKey &&
    !event.altKey &&
    (event.code === "KeyV" || event.key.toLowerCase() === "v")
  )
}
