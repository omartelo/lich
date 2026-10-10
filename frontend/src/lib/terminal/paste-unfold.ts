import type { SessionKind } from "@/lib/session/sessions"

// Claude Code folds a large paste into a "[Pasted text #1 +32 lines]" chip the
// user cannot edit, and unfolds it into the full text when the identical text
// is pasted again while the chip is still in the prompt. lich pastes twice
// whenever Claude Code would fold, so the text lands editable.
//
// The rule is Claude Code's own, read off the 2.1.296 bundle (its prompt's
// paste handler): fold above 800 characters or above min(rows - 10, 2)
// newlines, counted after it turns CRLF/CR into LF and a tab into four spaces;
// unfold only up to 100 000 characters. The second paste must match the first
// exactly, and writing both back to back was measured to unfold on a real PTY.
//
// Mirroring it is the point and the risk: a paste lich thinks folds but Claude
// Code does not lands twice (docs/ceilings.md).
const CLAUDE_FOLD_CHARS = 800
const CLAUDE_FOLD_NEWLINES = 2
const CLAUDE_FOLD_ROWS_RESERVED = 10
const CLAUDE_UNFOLD_MAX_CHARS = 100_000

function claudeFolds(text: string, rows: number): boolean {
  const normalized = text.replace(/\r\n|\r/g, "\n").replace(/\t/g, "    ")
  if (normalized.length > CLAUDE_UNFOLD_MAX_CHARS) {
    return false
  }
  const newlines = normalized.split("\n").length - 1
  const newlineLimit = Math.max(0, Math.min(rows - CLAUDE_FOLD_ROWS_RESERVED, CLAUDE_FOLD_NEWLINES))
  return normalized.length > CLAUDE_FOLD_CHARS || newlines > newlineLimit
}

/**
 * pasteTimes answers how many times a text paste must be written for it to
 * land in the session's prompt as editable text: 2 where the agent folds it
 * and unfolds on a repeat, 1 everywhere else. `rows` is the terminal's height,
 * which Claude Code's newline limit depends on.
 */
export function pasteTimes(kind: SessionKind, text: string, rows: number): 1 | 2 {
  if (kind === "claude" && claudeFolds(text, rows)) {
    return 2
  }
  return 1
}
