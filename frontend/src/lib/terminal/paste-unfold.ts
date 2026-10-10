import type { SessionKind } from "@/lib/session/sessions"

// Two providers fold a long paste into a placeholder the user cannot edit, and
// each unfolds it on a key of its own: Claude Code ("[Pasted text #1 +32
// lines]") when the identical text is pasted again while the chip is in the
// prompt, Kiro CLI ("32 lines ▸") on Tab. lich sends that key right after the
// paste wherever the provider would fold, so the text lands editable.
//
// Each rule is the provider's own, and mirroring it is the point and the risk:
// a paste lich thinks folds but the provider does not gets the key anyway
// (docs/ceilings.md).

// Claude Code 2.1.296, read off its prompt's paste handler: fold above 800
// characters or above min(rows - 10, 2) newlines, counted after it turns
// CRLF/CR into LF and a tab into four spaces; unfold only up to 100 000
// characters. Writing both pastes back to back was measured to unfold.
const CLAUDE_FOLD_CHARS = 800
const CLAUDE_FOLD_NEWLINES = 2
const CLAUDE_FOLD_ROWS_RESERVED = 10
const CLAUDE_UNFOLD_MAX_CHARS = 100_000

// Kiro CLI 2.21.0, read off its chat bundle (`shouldCollapse:t>10||a>500`) and
// measured at each edge: fold above 10 lines or above 500 UTF-16 units, counted
// on the text as the terminal delivers it, where every newline is one CR. A
// trailing newline opens an eleventh line. Tab expands the newest chip, and
// writing it right behind the paste was measured to unfold.
const KIRO_FOLD_LINES = 10
const KIRO_FOLD_CHARS = 500
const KIRO_UNFOLD_KEY = "\t"

function claudeFolds(text: string, rows: number): boolean {
  const normalized = text.replace(/\r\n|\r/g, "\n").replace(/\t/g, "    ")
  if (normalized.length > CLAUDE_UNFOLD_MAX_CHARS) {
    return false
  }
  const newlines = normalized.split("\n").length - 1
  const newlineLimit = Math.max(0, Math.min(rows - CLAUDE_FOLD_ROWS_RESERVED, CLAUDE_FOLD_NEWLINES))
  return normalized.length > CLAUDE_FOLD_CHARS || newlines > newlineLimit
}

function kiroFolds(text: string): boolean {
  const delivered = text.replace(/\r?\n/g, "\r")
  return delivered.split("\r").length > KIRO_FOLD_LINES || delivered.length > KIRO_FOLD_CHARS
}

/** What unfolds a paste: the same text pasted again, or a key written after it. */
export type PasteUnfold = { kind: "repaste" } | { kind: "key"; key: string }

/**
 * pasteUnfold answers what to send right after a text paste for it to land in
 * the session's prompt as editable text, or null where the agent keeps it as
 * typed. `rows` is the terminal's height, which Claude Code's newline limit
 * depends on.
 */
export function pasteUnfold(kind: SessionKind, text: string, rows: number): PasteUnfold | null {
  if (kind === "claude" && claudeFolds(text, rows)) {
    return { kind: "repaste" }
  }
  if (kind === "kiro" && kiroFolds(text)) {
    return { kind: "key", key: KIRO_UNFOLD_KEY }
  }
  return null
}
