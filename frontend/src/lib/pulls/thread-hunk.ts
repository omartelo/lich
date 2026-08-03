import { type DiffLine, type DiffSide, parseDiff } from "@/lib/git/diff"

// GitHub's diffHunk is the whole hunk up to the line the comment was filed on —
// on a file the branch adds, that is the entire file. What the thread is about
// is the tail of it, so the snippet is cut down to the commented span.

/** Lines of context kept above a thread that covers a single line. */
const CONTEXT = 3

/** The span a thread covers: what a ReviewThread carries about its anchor. */
export interface ThreadAnchor {
  line: number
  startLine: number
  side: string
}

/** How many lines a snippet shows at most, whatever the anchor says — a stray
 * span must not put the whole file back on screen. */
const MAX_LINES = 24

// threadHunk parses one GitHub diff hunk and returns the lines the thread was
// filed on, numbered. An anchor whose numbers the hunk does not hold (a thread
// GitHub re-anchored onto a rewritten file) falls back to the hunk's last lines:
// the hunk always ends on the commented line, so its tail is the span even when
// the numbers have moved.
export function threadHunk(diffHunk: string, anchor: ThreadAnchor): DiffLine[] {
  // parseDiff wants a file header above the hunk; the path is never read back.
  const [file] = parseDiff(`diff --git a/hunk b/hunk\n${diffHunk}`)
  const lines = file?.hunks[0]?.lines ?? []
  const side: DiffSide = anchor.side === "LEFT" ? "LEFT" : "RIGHT"
  const first = anchor.startLine > 0 ? anchor.startLine : anchor.line
  const at = lines.findIndex((line) => {
    const number = side === "LEFT" ? line.oldLine : line.newLine
    return number !== null && number >= first
  })
  // A span states its own first line; a single-line thread reads better with a
  // few lines above it than alone.
  const from = at === -1 ? lines.length - 1 - CONTEXT : at - (anchor.startLine > 0 ? 0 : CONTEXT)
  return lines.slice(Math.max(0, from, lines.length - MAX_LINES))
}
