// The session sidebar's own filter: which of a project's cards survive the
// query typed into it. Kept here rather than in the component because the
// frontend suite runs in node and cannot render one — and this is the half with
// a rule in it.
//
// It is not the command palette. The palette spans every open project and
// navigates once; this narrows one sidebar and is held while the work happens,
// so the surviving cards stay live — status ring, tool rung, close, delegate.

import { matchesQuery } from "./command-palette"
import type { Session } from "./sessions"

export interface SidebarFilter {
  // The cards the sidebar draws, in the stored order: the matches, plus the
  // active session whatever the query says.
  sessions: Session[]
  // Whether anything actually matched. Not derivable from `sessions` being
  // empty — the kept active card means it rarely is — and it is what tells the
  // empty state apart from a narrowed list.
  matched: boolean
}

// filterSessions narrows a project's sessions to the ones whose label or
// checkout matches the query. The stored order is preserved, so grouping, the
// pinned block and the drag order downstream all keep working on the survivors
// without knowing a filter exists.
//
// The active session survives whatever the query says: the lit card is the
// identity of the terminal beside it, and a sidebar with nothing lit while a
// session is on screen reads as broken. SidebarRail keeps its highlight on the
// full-screen routes for the same reason.
//
// The haystack is the label and the checkout, which is what makes typing a
// worktree name narrow to that checkout — the trick FilesPanel already relies
// on for directory names. The branch is deliberately not in it: it is read per
// card by useGitStatus, and hoisting that into the sidebar to filter on it buys
// little the checkout path does not already give.
//
// A blank query hands the input array straight back, so the ordinary case costs
// nothing.
export function filterSessions(
  sessions: Session[],
  query: string,
  projectPath: string,
  activeId: string,
): SidebarFilter {
  if (query.trim() === "") {
    return { sessions, matched: true }
  }
  const hit = (session: Session) =>
    matchesQuery(`${session.label} ${session.path || projectPath}`, query)
  return {
    sessions: sessions.filter((session) => session.id === activeId || hit(session)),
    matched: sessions.some(hit),
  }
}
