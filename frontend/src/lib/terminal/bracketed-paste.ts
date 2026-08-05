// Bracketed paste: the escape pair a terminal wraps hand-pasted text in, so a
// TUI reads the block as one paste instead of typing. It is what keeps a
// newline inside the block from being a send, and it leaves the text unsent at
// the prompt — the user presses Enter.

const PASTE_START = "\x1b[200~"
const PASTE_END = "\x1b[201~"

export function bracketedPaste(text: string): string {
  return `${PASTE_START}${text}${PASTE_END}`
}
