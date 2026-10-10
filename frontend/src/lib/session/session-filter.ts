// The session sidebar's own filter: which of a project's cards survive the
// query typed into it. Kept here rather than in the component because the
// frontend suite runs in node and cannot render one — and this is the half with
// a rule in it.
//
// It is not the command palette. The palette spans every open project and
// navigates once; this narrows one sidebar and is held while the work happens,
// so the surviving cards stay live — status ring, tool rung, close, delegate.

import { matchesQuery } from "./command-palette"
import type { SessionStatus } from "./session-events"
import type { Session } from "./sessions"
import { t } from "@/lib/i18n/i18n"

// The states the filter's chips offer, in the order they are drawn: what blocks
// the user first, then what is still moving, then what is new, then the rest.
export const SESSION_PHASES = ["waiting", "running", "unread", "idle"] as const
export type SessionPhase = (typeof SESSION_PHASES)[number]

// phaseOf files a session under one chip from what its ring shows. A read turn
// is idle rather than a chip of its own: its faded ring already says nothing is
// happening and nothing is new, which is the question idle answers. A session
// whose provider never reports (Crush, Cursor CLI, a plain shell) is idle for
// the same reason it has no ring.
export function phaseOf(status: SessionStatus | null, unread: boolean): SessionPhase {
  switch (status) {
    case "waiting":
      return "waiting"
    case "busy":
    case "compacting":
      return "running"
    case "done":
      return unread ? "unread" : "idle"
    default:
      return "idle"
  }
}

export interface SidebarFilter {
  // The cards the sidebar draws, in the stored order: the matches, plus the
  // active session whatever the query says.
  sessions: Session[]
  // Whether anything actually matched. Not derivable from `sessions` being
  // empty — the kept active card means it rarely is — and it is what tells the
  // empty state apart from a narrowed list.
  matched: boolean
}

const queryHit = (query: string, projectPath: string) => (session: Session) =>
  matchesQuery(`${session.label} ${session.path || projectPath}`, query)

// filterSessions narrows a project's sessions to the ones whose label or
// checkout matches the query and whose phase is one of `phases`; an empty set
// picks no phase out, so it narrows nothing. The stored order is preserved, so
// grouping, the pinned block and the drag order downstream all keep working on
// the survivors without knowing a filter exists.
//
// The active session survives whatever the filter says: the lit card is the
// identity of the terminal beside it, and a sidebar with nothing lit while a
// session is on screen reads as broken. SidebarRail keeps its highlight on the
// full-screen routes for the same reason. It is also what keeps the card being
// answered on screen while a phase filter is on: a waiting session the user
// replies to turns running under the cursor and would otherwise vanish.
//
// The haystack is the label and the checkout, which is what makes typing a
// worktree name narrow to that checkout — the trick FilesPanel already relies
// on for directory names. The branch is deliberately not in it: it is read per
// card by useGitStatus, and hoisting that into the sidebar to filter on it buys
// little the checkout path does not already give.
//
// A blank query with no phase picked hands the input array straight back, so
// the ordinary case costs nothing.
export function filterSessions(
  sessions: Session[],
  query: string,
  projectPath: string,
  activeId: string,
  phases: ReadonlySet<SessionPhase>,
  phaseOfSession: (id: string) => SessionPhase,
): SidebarFilter {
  const blank = query.trim() === ""
  if (blank && phases.size === 0) {
    return { sessions, matched: true }
  }
  const textHit = queryHit(query, projectPath)
  const hit = (session: Session) =>
    (blank || textHit(session)) && (phases.size === 0 || phases.has(phaseOfSession(session.id)))
  return {
    sessions: sessions.filter((session) => session.id === activeId || hit(session)),
    matched: sessions.some(hit),
  }
}

// phaseCounts is the number beside each chip: the sessions the query matches,
// per phase. Counted against the query alone and never against the chips, so a
// count says what picking that chip would add rather than going to zero the
// moment another chip is picked.
export function phaseCounts(
  sessions: Session[],
  query: string,
  projectPath: string,
  phaseOfSession: (id: string) => SessionPhase,
): Record<SessionPhase, number> {
  const counts: Record<SessionPhase, number> = { waiting: 0, running: 0, unread: 0, idle: 0 }
  const hit = query.trim() === "" ? () => true : queryHit(query, projectPath)
  for (const session of sessions) {
    if (hit(session)) {
      counts[phaseOfSession(session.id)]++
    }
  }
  return counts
}

// noMatchNotice is the sentence under an empty filter. It names the phases
// picked in chip order, whatever order they were clicked in, and ends on the one
// card still there, which is why it is not simply "no results".
export function noMatchNotice(query: string, phases: ReadonlySet<SessionPhase>): string {
  const trimmed = query.trim()
  const picked = SESSION_PHASES.filter((phase) => phases.has(phase))
    .map((phase) => t(`session.filter.phase.${phase}`))
    .join(t("session.filter.phaseJoiner"))
  if (picked === "") {
    return trimmed === ""
      ? t("session.filter.noMatch")
      : t("session.filter.noMatchQuery", { query: trimmed })
  }
  return trimmed === ""
    ? t("session.filter.noMatchPhases", { phases: picked })
    : t("session.filter.noMatchPhasesQuery", { phases: picked, query: trimmed })
}
