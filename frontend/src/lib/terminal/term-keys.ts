// Key chords xterm.js encodes differently from what our TUIs expect. The
// sequences go straight to the PTY through attachCustomKeyEventHandler,
// bypassing xterm's own encoder:
//
// - Ctrl+Backspace: xterm sends BS (\x08), a single-char erase. Send ETB
//   (\x17, readline's unix-word-rubout) so line editors erase the word.
// - Ctrl+V: in Chromium the default action pastes text into the terminal,
//   hiding the keypress from TUIs that read the clipboard themselves on ^V
//   (Claude Code image attach). Send SYN (\x16) like a real terminal — but on
//   Windows Claude Code binds image-paste to Alt+V (ESC+v), not ^V, so emit
//   that there instead. Text paste stays on Ctrl+Shift+V (Cmd+V on macOS),
//   the browser's own paste; one carrying an image and no text is turned into
//   the same chord (pastedImageSequence).
// - Shift+Enter: xterm sends plain \r, indistinguishable from Enter. ESC+CR
//   is what TUIs (Claude Code) accept as "insert newline" without
//   kitty-protocol negotiation.
//
// Everything else (Shift+Tab → CSI Z, Alt chords, SGR wheel reports)
// xterm.js encodes correctly on its own — only the three above need help.

// The subset of KeyboardEvent the matcher needs — lets tests pass plain
// objects.
export type TermKeyState = Pick<
  KeyboardEvent,
  "ctrlKey" | "metaKey" | "shiftKey" | "altKey" | "key"
> & { code?: string }

// isSearchOpenChord reports Ctrl+F with no other modifier — the in-terminal
// search trigger. It lives here beside chordSequence so all terminal key-chord
// decisions stay in one tested place, but unlike those it is not a PTY sequence:
// the caller opens the search box instead of writing to the shell (shadowing the
// shell's forward-char, the same trade VS Code's terminal makes).
export function isSearchOpenChord(event: TermKeyState): boolean {
  return (
    event.ctrlKey &&
    !event.metaKey &&
    !event.altKey &&
    !event.shiftKey &&
    event.key.toLowerCase() === "f"
  )
}

function imageAttachSequence(isWindows: boolean): string {
  return isWindows ? "\x1bv" : "\x16"
}

// pastedImageSequence answers a paste the terminal would drop: xterm only
// reads text, and a screenshot on the clipboard has none. A macOS user pastes
// with Cmd+V, never Ctrl+V, so this is what reaches Claude Code's image attach
// there. The agent reads the image off the clipboard itself.
export function pastedImageSequence(
  data: Pick<DataTransfer, "types" | "getData"> | null,
  isWindows = false,
): string | null {
  if (data === null || data.getData("text/plain") !== "" || !data.types.includes("Files")) {
    return null
  }
  return imageAttachSequence(isWindows)
}

export function chordSequence(event: TermKeyState, isWindows = false): string | null {
  const ctrlOnly = event.ctrlKey && !event.metaKey && !event.altKey && !event.shiftKey
  if (ctrlOnly && event.key === "Backspace") {
    return "\x17"
  }
  if (ctrlOnly && (event.code === "KeyV" || event.key.toLowerCase() === "v")) {
    return imageAttachSequence(isWindows)
  }
  if (
    event.key === "Enter" &&
    event.shiftKey &&
    !event.altKey &&
    !event.ctrlKey &&
    !event.metaKey
  ) {
    return "\x1b\r"
  }
  return null
}
