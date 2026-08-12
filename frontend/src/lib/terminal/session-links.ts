// Turns the open sessions (session/command-palette.ts's own flatten of every
// session across every project) into a plain-text matcher for the terminal's
// custom link provider: another session's label, printed in this terminal's
// output, becomes a clickable jump to it.
//
// A label shared by more than one open session is dropped rather than guessed
// at — the terminal has no way to tell which one the printed text meant.

import type { PaletteSession } from "@/lib/session/command-palette"

// A label links only where it stands as a word of its own: never glued to a
// neighbouring word ("auth" inside "authentication"), never a piece of a path
// or filename ("auth" inside "src/auth.ts"). Both are common enough in a
// provider's output that without this a one-word session title underlines half
// the screen, and a path is the one thing the link must not claim — the
// terminal already has no link for it, so a wrong one reads as a real one.
const LEFT_BOUNDARY = String.raw`(?<![\w./\\-])`
const RIGHT_BOUNDARY = String.raw`(?![\w-])(?!\.\w)(?![/\\])`

// SessionLinkTargets is the label→session map together with the matcher built
// from its keys. The pattern is built once per roster change rather than per
// hovered row: xterm asks a provider for links on every mouse move, so building
// the alternation inside the match would rebuild it thousands of times for a
// list that changes when a session opens or closes.
export interface SessionLinkTargets {
  byLabel: ReadonlyMap<string, PaletteSession>
  /** Null when there is nothing to match, so a caller can skip reading the buffer. */
  pattern: RegExp | null
}

export function sessionLinkTargets(
  sessions: readonly PaletteSession[],
  excludeId: string,
): SessionLinkTargets {
  const seen = new Map<string, PaletteSession | null>()
  for (const session of sessions) {
    // An empty label would put an empty alternative in the pattern, which
    // matches at every position without advancing lastIndex — a hung tab on
    // hover, not a missing link. Nothing produces one today (a rename commits a
    // trimmed non-empty value, an ai-title POST is refused empty), so this is a
    // guard against the hang, not a case with a behaviour to define.
    if (session.sessionId === excludeId || session.label === "") {
      continue
    }
    seen.set(session.label, seen.has(session.label) ? null : session)
  }
  const byLabel = new Map<string, PaletteSession>()
  for (const [label, session] of seen) {
    if (session) {
      byLabel.set(label, session)
    }
  }
  return { byLabel, pattern: buildPattern([...byLabel.keys()]) }
}

export interface LabelMatch {
  start: number
  end: number
  session: PaletteSession
}

function escapeRegExp(text: string): string {
  return text.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")
}

function buildPattern(labels: string[]): RegExp | null {
  if (labels.length === 0) {
    return null
  }
  // Longest label first so "auth service" wins over "auth" at the same position
  // instead of splitting it. The boundaries sit outside the group, so a longer
  // label rejected by them falls back to a shorter one that passes.
  const alternation = labels
    .sort((a, b) => b.length - a.length)
    .map(escapeRegExp)
    .join("|")
  return new RegExp(`${LEFT_BOUNDARY}(?:${alternation})${RIGHT_BOUNDARY}`, "g")
}

// findLabelMatches finds every non-overlapping occurrence of a target's label in
// a line of plain text.
export function findLabelMatches(lineText: string, targets: SessionLinkTargets): LabelMatch[] {
  const { byLabel, pattern } = targets
  if (!pattern || lineText === "") {
    return []
  }
  const matches: LabelMatch[] = []
  // The pattern outlives the call now, so the walk has to start from the top
  // rather than from wherever the previous line left it.
  pattern.lastIndex = 0
  let match: RegExpExecArray | null
  // biome-ignore lint/suspicious/noAssignInExpressions: the RegExp#exec loop idiom.
  while ((match = pattern.exec(lineText))) {
    // Total by construction: the alternation is built from byLabel's own keys
    // and the boundaries are lookarounds, so the whole match is always a label.
    const session = byLabel.get(match[0]) as PaletteSession
    matches.push({ start: match.index, end: match.index + match[0].length, session })
  }
  return matches
}
