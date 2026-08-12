// Turns the open sessions (session/command-palette.ts's own flatten of every
// session across every project) into a plain-text matcher for the terminal's
// custom link provider: another session's label, printed in this terminal's
// output, becomes a clickable jump to it.
//
// A label shared by more than one open session is dropped rather than guessed
// at — the terminal has no way to tell which one the printed text meant.

import type { PaletteSession } from "@/lib/session/command-palette"

export function sessionLinkTargets(
  sessions: readonly PaletteSession[],
  excludeId: string,
): ReadonlyMap<string, PaletteSession> {
  const byLabel = new Map<string, PaletteSession | null>()
  for (const session of sessions) {
    if (session.sessionId === excludeId) {
      continue
    }
    byLabel.set(session.label, byLabel.has(session.label) ? null : session)
  }
  const targets = new Map<string, PaletteSession>()
  for (const [label, session] of byLabel) {
    if (session) {
      targets.set(label, session)
    }
  }
  return targets
}

export interface LabelMatch {
  start: number
  end: number
  session: PaletteSession
}

function escapeRegExp(text: string): string {
  return text.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")
}

// findLabelMatches finds every non-overlapping occurrence of a target's label
// in a line of plain text, longest label first so "auth service" wins over
// "auth" at the same position instead of splitting it.
export function findLabelMatches(
  lineText: string,
  targets: ReadonlyMap<string, PaletteSession>,
): LabelMatch[] {
  if (targets.size === 0 || lineText === "") {
    return []
  }
  const labels = [...targets.keys()].sort((a, b) => b.length - a.length)
  const pattern = new RegExp(labels.map(escapeRegExp).join("|"), "g")
  const matches: LabelMatch[] = []
  let match: RegExpExecArray | null
  // biome-ignore lint/suspicious/noAssignInExpressions: the RegExp#exec loop idiom.
  while ((match = pattern.exec(lineText))) {
    const session = targets.get(match[0])
    if (session) {
      matches.push({ start: match.index, end: match.index + match[0].length, session })
    }
  }
  return matches
}
