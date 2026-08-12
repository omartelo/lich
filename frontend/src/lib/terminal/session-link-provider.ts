// Wraps findLabelMatches (session-links.ts) as an xterm ILinkProvider: another
// open session's label, printed in this terminal's output, becomes a
// clickable jump to it — the same custom-link-provider mechanism
// WebLinksAddon (TerminalView.tsx) uses for URLs, but xterm has none built in
// for arbitrary text, so this is a small hand-written one.
//
// ponytail: matches are found one physical row at a time, one string
// character costing exactly one cell — a label that wraps across two rows, or
// carries a wide/combining character, is missed. Session labels are short,
// mostly-ASCII, user-typed slugs in practice; if that ever stops holding,
// @xterm/addon-web-links's internal LinkComputer shows the fuller wrapped-line
// + cell-width walk to grow into.

import type { ILink, ILinkProvider, Terminal } from "@xterm/xterm"
import type { PaletteSession } from "@/lib/session/command-palette"
import { findLabelMatches } from "./session-links"

export function createSessionLinkProvider(
  term: Terminal,
  targetsRef: { current: ReadonlyMap<string, PaletteSession> },
  activate: (target: PaletteSession) => void,
): ILinkProvider {
  return {
    provideLinks(bufferLineNumber, callback) {
      const targets = targetsRef.current
      const line = targets.size > 0 ? term.buffer.active.getLine(bufferLineNumber - 1) : undefined
      if (!line) {
        callback(undefined)
        return
      }
      const text = line.translateToString(true)
      const matches = findLabelMatches(text, targets)
      if (matches.length === 0) {
        callback(undefined)
        return
      }
      const links: ILink[] = matches.map(({ start, end, session }) => ({
        range: {
          start: { x: start + 1, y: bufferLineNumber },
          end: { x: end, y: bufferLineNumber },
        },
        text: text.slice(start, end),
        decorations: { pointerCursor: true, underline: true },
        activate: () => activate(session),
      }))
      callback(links)
    },
  }
}
