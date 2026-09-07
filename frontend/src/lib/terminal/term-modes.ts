/**
 * Terminal modes a snapshot cannot carry.
 *
 * Hiding a session serializes the terminal and destroys it; showing it writes
 * the snapshot into a fresh one. xterm's SerializeAddon replays the modes it
 * can read off `term.modes`, and three an app depends on are not among them:
 *
 * - The mouse *encoding*. The addon restores the mouse *protocol*
 *   (`?1000`/`?1002`/`?1003`) but never the encoding the app turned on with it,
 *   so the rebuilt terminal falls back to the legacy X10 encoding — which xterm
 *   reports on `onBinary`, a channel the transport does not carry. The app then
 *   sees no clicks, no scroll and no drag at all until it happens to re-emit
 *   its own encoding, while the keyboard keeps working.
 * - Cursor visibility (`?25`). A fresh terminal shows its cursor, so an app
 *   that hid it — every full-screen TUI does, drawing its own — gets lich's
 *   blinking block back, parked wherever the snapshot left the real cursor.
 * - Cursor shape (DECSCUSR, `CSI Ps SP q`). xterm parses it into a private the
 *   addon never reads, so a shell or editor that asked for a bar or an
 *   underline gets lich's block back on the next card switch.
 */

// Keyed by xterm's CoreMouseEncoding names; DEFAULT (X10) needs no sequence.
const MOUSE_ENCODING_SEQUENCES: Record<string, string> = {
  SGR: "\x1b[?1006h",
  SGR_PIXELS: "\x1b[?1016h",
  UTF8: "\x1b[?1005h",
  URXVT: "\x1b[?1015h",
}

/**
 * The DECSET sequence that re-selects a mouse encoding, or "" when there is
 * nothing to restore (no encoding read, X10, or a name xterm added since).
 */
export function mouseEncodingSequence(encoding: string | undefined): string {
  return encoding ? (MOUSE_ENCODING_SEQUENCES[encoding] ?? "") : ""
}

/**
 * The DECRST sequence that hides the cursor again, or "" when it was visible —
 * which is what a fresh terminal already is.
 */
export function cursorVisibilitySequence(hidden: boolean): string {
  return hidden ? "\x1b[?25l" : ""
}

// Keyed by xterm's cursorStyle names; each pair is the DECSCUSR Ps for that
// shape, blinking first. A style xterm never set means the app never chose one
// (or reset it with Ps 0 / RIS), which is the fresh terminal's own default.
const CURSOR_SHAPE_PS: Record<string, [blinking: number, steady: number]> = {
  block: [1, 2],
  underline: [3, 4],
  bar: [5, 6],
}

/**
 * The DECSCUSR sequence that re-selects the cursor shape the app asked for, or
 * "" when it never asked, reset it, or picked a name xterm added since.
 */
export function cursorShapeSequence(shape: { style?: string; blink?: boolean }): string {
  const ps = shape.style ? CURSOR_SHAPE_PS[shape.style] : undefined
  return ps ? `\x1b[${shape.blink === false ? ps[1] : ps[0]} q` : ""
}

/**
 * Whether a click on a link is ours to open: Ctrl+Click, or Cmd+Click on macOS.
 * The click is also swallowed before it reaches the PTY (TerminalView), because
 * an app that reads the mouse and opens its own links (Claude Code does) would
 * otherwise serve the same click a second time, one browser tab each.
 */
export function linkClickIsOurs(event: Pick<MouseEvent, "ctrlKey" | "metaKey">): boolean {
  return event.ctrlKey || event.metaKey
}
