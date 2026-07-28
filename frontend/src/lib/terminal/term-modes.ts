/**
 * Terminal modes a snapshot cannot carry.
 *
 * Hiding a session serializes the terminal and destroys it; showing it writes
 * the snapshot into a fresh one. xterm's SerializeAddon restores the mouse
 * *protocol* (`?1000`/`?1002`/`?1003`) but never the *encoding* the app turned
 * on with it, so the rebuilt terminal falls back to the legacy X10 encoding —
 * which xterm reports on `onBinary`, a channel the transport does not carry.
 * The app then sees no clicks, no scroll and no drag at all until it happens to
 * re-emit its own encoding, while the keyboard keeps working.
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
