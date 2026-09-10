// Wraps findLabelMatches (session-links.ts) as an xterm ILinkProvider: another
// open session's label, printed in this terminal's output, becomes a
// clickable jump to it — the same custom-link-provider mechanism
// WebLinksAddon (TerminalView.tsx) uses for URLs, but xterm has none built in
// for arbitrary text, so this is a small hand-written one.

import type { ILink, ILinkProvider, Terminal } from "@xterm/xterm"
import type { PaletteSession } from "@/lib/session/command-palette"
import { findLabelMatches, type SessionLinkTargets } from "./session-links"

// The match runs over the whole logical line, continuation rows included, so a
// label that wrapped still links. The walk stops here in either direction: a
// program printing megabytes without a newline must not turn every mouse move
// into a scan of the scrollback.
const MAX_WINDOW_CELLS = 2048

// A logical line as one string, plus the cells each string index came from.
// Neither direction is 1:1 — a wide char covers two cells, a combining mark
// adds a char without adding a cell, and a wrap moves the next char to a row of
// its own — so the mapping is recorded during the walk rather than computed
// back from a string offset.
interface WindowedLine {
  text: string
  /** Buffer row of the char at each string index. */
  row: number[]
  /** First cell of the char at each string index, 0-based. */
  cellStart: number[]
  /** Cell after the char at each string index, 0-based. */
  cellEnd: number[]
}

// readWindow reads the whole logical line the given buffer row belongs to.
// Continuation rows are the ones flagged isWrapped, which is what xterm sets
// when the cursor auto-wrapped rather than met a newline.
function readWindow(term: Terminal, lineIndex: number): WindowedLine | null {
  const buf = term.buffer.active
  if (!buf.getLine(lineIndex)) {
    return null
  }
  let budget = MAX_WINDOW_CELLS
  let top = lineIndex
  while (budget > 0 && buf.getLine(top)?.isWrapped) {
    const above = buf.getLine(top - 1)
    if (!above) {
      break
    }
    top--
    budget -= above.length
  }
  budget = MAX_WINDOW_CELLS
  let bottom = lineIndex
  while (budget > 0) {
    const below = buf.getLine(bottom + 1)
    if (!below?.isWrapped) {
      break
    }
    bottom++
    budget -= below.length
  }

  const cell = buf.getNullCell()
  const line: WindowedLine = { text: "", row: [], cellStart: [], cellEnd: [] }
  for (let y = top; y <= bottom; y++) {
    const row = buf.getLine(y)
    if (!row) {
      break
    }
    for (let x = 0; x < row.length; ) {
      row.getCell(x, cell)
      // A cell holding no codepoint still owns its column, and reads back as
      // the space translateToString would have put there. Advancing by the
      // width steps over the right half of a wide char, which carries no
      // character of its own; the fallback of 1 is what keeps a zero-width
      // cell from stalling the walk, the same guard xterm's own walk has.
      const chars = cell.getChars() || " "
      const next = x + (cell.getWidth() || 1)
      line.text += chars
      for (let i = 0; i < chars.length; i++) {
        line.row.push(y)
        line.cellStart.push(x)
        line.cellEnd.push(next)
      }
      x = next
    }
  }
  return line
}

export function createSessionLinkProvider(
  term: Terminal,
  targetsRef: { current: SessionLinkTargets },
  activate: (target: PaletteSession) => void,
): ILinkProvider {
  return {
    provideLinks(bufferLineNumber, callback) {
      const targets = targetsRef.current
      const line = targets.pattern ? readWindow(term, bufferLineNumber - 1) : null
      if (!line) {
        callback(undefined)
        return
      }
      const matches = findLabelMatches(line.text, targets)
      if (matches.length === 0) {
        callback(undefined)
        return
      }
      const links: ILink[] = matches.map(({ start, end, session }) => ({
        // xterm's range is 1-based and includes its right edge, so the first
        // cell shifts up by one while the exclusive end cell already names the
        // last column. Taking the end from the match's last char, rather than
        // from the one past it, is what keeps a label ending flush with the
        // right margin from reporting column 0 of the row below.
        range: {
          start: { x: line.cellStart[start] + 1, y: line.row[start] + 1 },
          end: { x: line.cellEnd[end - 1], y: line.row[end - 1] + 1 },
        },
        text: line.text.slice(start, end),
        decorations: { pointerCursor: true, underline: true },
        activate: () => activate(session),
      }))
      callback(links)
    },
  }
}
