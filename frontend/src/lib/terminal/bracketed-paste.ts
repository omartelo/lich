// Bracketed paste: the escape pair a terminal wraps hand-pasted text in, so a
// TUI reads the block as one paste instead of typing. It is what keeps a
// newline inside the block from being a send, and it leaves the text unsent at
// the prompt — the user presses Enter.

const PASTE_START = "\x1b[200~"
const PASTE_END = "\x1b[201~"

// strip is the rule itself, written the way internal/relay/compose.go sanitize
// writes it so the two halves of the same door can be read side by side. keep
// is all the two frontend write paths disagree on, so it is all either passes.
function strip(text: string, keep: string): string {
  let out = ""
  for (const char of text) {
    const code = char.codePointAt(0) ?? 0
    if (keep.includes(char) || (code >= 0x20 && code !== 0x7f)) {
      out += char
    }
  }
  return out
}

/**
 * Wraps text as one bracketed paste, neutralising anything in it that a
 * terminal would act on rather than display.
 *
 * This is a trust boundary, not a formatting helper. What the callers hand it
 * is written by strangers: a GitHub issue body (lib/issue.ts), a review comment
 * or a CI log (lib/review-comments.ts, lib/pulls/pr-handoff.ts), a dropped
 * file's name or path (terminal/drop-files.ts). ESC is the lever: "ESC[201~"
 * inside the text closes the block early, so every byte after it reaches the
 * agent as keys it runs rather than a prompt the user still has to send. Anyone
 * who can file an issue on a public repository can write those bytes, and
 * quoting does not help, because the sequence is consumed by the terminal
 * before a shell ever sees it.
 *
 * The rule mirrors internal/relay/compose.go sanitize, which is the same guard
 * on the backend's own way into a session's prompt: keep \n and \t, drop the
 * rest of C0 and DEL. Named here so the two cannot drift apart quietly — a
 * character that stops being safe stops being safe on both sides.
 *
 *   - \n stays, because carrying a multi-line body unsent is the whole reason
 *     these call sites paste at all, and inside the block it is text.
 *   - \t stays, because an indented code block in a body is not an attack.
 *   - \r goes: on its own it is a submit in some readers, which is the same
 *     escape from "unsent" by another door, and dropping it leaves a CRLF body
 *     as the plain one it meant to be.
 *
 * Dropped rather than replaced, because the text is usually prose and a marker
 * of lich's own in the middle of an issue body reads as the body's content.
 */
export function bracketedPaste(text: string): string {
  return `${PASTE_START}${strip(text, "\n\t")}${PASTE_END}`
}

/**
 * The same rule for the other way into a PTY: useInject, which writes what it
 * is given with nothing around it.
 *
 * Nothing wraps that text, so a newline in it is a send rather than a line, and
 * the three call sites all inject one file or line reference ending in a space
 * (components/diff). A reference that spans lines is not one, so \n and \t go
 * here as well: the strict rule is the default, and bracketedPaste is the one
 * place that relaxes it.
 *
 * A caller may hand over a block bracketedPaste already built (a batch of
 * review comments). Stripping that outright would eat the wrapper's own escapes
 * and turn the block back into typing, so it is unwrapped and wrapped again,
 * which re-runs the rule over the inside rather than trusting it.
 */
export function safeToWrite(text: string): string {
  if (text.startsWith(PASTE_START) && text.endsWith(PASTE_END)) {
    return bracketedPaste(text.slice(PASTE_START.length, -PASTE_END.length))
  }
  return strip(text, "")
}
