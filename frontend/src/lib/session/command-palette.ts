// The command palette's data join: open projects and their sessions become a
// flat, filterable list the palette lists and routes to. Kept pure (no React,
// no stores) so the flatten and filter are testable without a render.

import type { Project, TranscriptMatch } from "@/lib/api-types"
import type { SessionKind, SessionState } from "./sessions"

// PaletteSession is one session flattened with the project it belongs to — what
// a "jump to session" row shows and routes to.
export interface PaletteSession {
  sessionId: string
  projectId: string
  projectName: string
  label: string
  kind: SessionKind
  // Where the session runs: its own worktree path, else the project's path.
  path: string
}

// paletteSessions flattens every session of the open projects. Sessions whose
// project has no open tab are dropped — they are not reachable by navigation,
// the same rule the notification queue applies.
export function paletteSessions(
  projects: readonly Project[],
  sessions: SessionState,
): PaletteSession[] {
  const rows: PaletteSession[] = []
  for (const project of projects) {
    const entry = sessions[project.id]
    if (!entry) {
      continue
    }
    for (const session of entry.sessions) {
      rows.push({
        sessionId: session.id,
        projectId: project.id,
        projectName: project.name,
        label: session.label,
        kind: session.kind,
        path: session.path ?? project.path,
      })
    }
  }
  return rows
}

// matchesQuery reports whether every whitespace-separated token of the query
// appears in haystack, case-insensitively — a light fuzzy filter, so "revu auth"
// matches "Wire revu auth middleware". An empty query matches everything.
export function matchesQuery(haystack: string, query: string): boolean {
  const q = query.trim().toLowerCase()
  if (q === "") {
    return true
  }
  const hay = haystack.toLowerCase()
  return q.split(/\s+/).every((token) => hay.includes(token))
}

export interface PaletteResults {
  sessions: PaletteSession[]
  projects: Project[]
  // The closed projects, which the reopen menu caps at five and this list does
  // not: past that cap the palette is the only way back to one.
  closed: Project[]
}

export function filterPalette(
  query: string,
  allSessions: readonly PaletteSession[],
  projects: readonly Project[],
  closed: readonly Project[] = [],
): PaletteResults {
  return {
    sessions: allSessions.filter((s) =>
      matchesQuery(`${s.label} ${s.projectName} ${s.path}`, query),
    ),
    projects: projects.filter((p) => matchesQuery(`${p.name} ${p.path}`, query)),
    closed: closed.filter((p) => matchesQuery(`${p.name} ${p.path}`, query)),
  }
}

// MESSAGE_MIN_QUERY mirrors the backend's minSearchQuery: under three
// characters the search is refused there, so the palette does not ask for it.
export const MESSAGE_MIN_QUERY = 3

// PaletteMessage is a session the query was talked about in — the same row a
// "jump to session" is, carrying the sentence that made it a hit.
export interface PaletteMessage extends PaletteSession {
  snippet: string
  count: number
}

// paletteMessages joins transcript matches back onto the sessions they were
// searched for, keeping the order the search returned. A match whose session
// has closed since the query went out is dropped: the row would have nowhere to
// jump to. Null (a search that found nothing, or never ran) is no rows.
export function paletteMessages(
  matches: readonly TranscriptMatch[] | null,
  sessions: readonly PaletteSession[],
): PaletteMessage[] {
  if (!matches) {
    return []
  }
  const rows: PaletteMessage[] = []
  for (const match of matches) {
    const session = sessions.find((s) => s.sessionId === match.id)
    if (session) {
      rows.push({ ...session, snippet: match.snippet, count: match.count })
    }
  }
  return rows
}

// PALETTE_TABS is the filter row, in the order Tab walks it. A query can hit
// four kinds of thing at once, which is a page of rows nobody reads: All keeps
// every kind but shows the first few of each, and a tab lists one kind whole.
export const PALETTE_TABS = ["All", "Sessions", "Projects", "Messages"] as const

export type PaletteTab = (typeof PALETTE_TABS)[number]

// ALL_TAB_ROWS is how much of a group the All tab shows. Three is enough for the
// hit that is obviously the one and short enough that four groups still fit
// without scrolling.
export const ALL_TAB_ROWS = 3

// PaletteRow is one option in the list, carrying what running it needs. The
// groups are rendered in order and the rows flattened back out for the arrow
// keys, so a row's index is its position in that flat list and nothing else.
export type PaletteRow =
  | { kind: "session"; session: PaletteSession }
  | { kind: "project"; project: Project }
  | { kind: "closed"; project: Project }
  | { kind: "message"; message: PaletteMessage }

export interface PaletteGroup {
  label: string
  rows: PaletteRow[]
  // Rows the group holds before the All tab's cut, so its header can say what
  // it is leaving out. Equal to rows.length when nothing was cut.
  total: number
}

// rowKey identifies a row inside the list it is rendered in. A session and a
// message about it share an id, and they never share a group.
export function rowKey(row: PaletteRow): string {
  return row.kind === "session"
    ? row.session.sessionId
    : row.kind === "message"
      ? row.message.sessionId
      : row.project.id
}

// cap of 0 means no cap — the tab that names one kind lists all of it.
function group(label: string, rows: PaletteRow[], cap: number): PaletteGroup {
  return { label, rows: cap > 0 ? rows.slice(0, cap) : rows, total: rows.length }
}

export function paletteGroups(
  tab: PaletteTab,
  results: PaletteResults,
  messages: readonly PaletteMessage[],
): PaletteGroup[] {
  const sessions = results.sessions.map((session): PaletteRow => ({ kind: "session", session }))
  const open = results.projects.map((project): PaletteRow => ({ kind: "project", project }))
  const closed = results.closed.map((project): PaletteRow => ({ kind: "closed", project }))
  const said = messages.map((message): PaletteRow => ({ kind: "message", message }))

  const groups = ((): PaletteGroup[] => {
    switch (tab) {
      case "Sessions":
        return [group("Sessions", sessions, 0)]
      case "Projects":
        return [group("Open", open, 0), group("Closed", closed, 0)]
      case "Messages":
        return [group("Messages", said, 0)]
      default:
        return [
          group("Sessions", sessions, ALL_TAB_ROWS),
          group("Projects", open, ALL_TAB_ROWS),
          group("Closed projects", closed, ALL_TAB_ROWS),
          group("Messages", said, ALL_TAB_ROWS),
        ]
    }
  })()
  return groups.filter((g) => g.rows.length > 0)
}

// paletteTabCount is what a tab reports it holds. All reports nothing: it is
// every other tab added up, and the number would only restate the list below it.
export function paletteTabCount(
  tab: PaletteTab,
  results: PaletteResults,
  messages: readonly PaletteMessage[],
): number | null {
  switch (tab) {
    case "Sessions":
      return results.sessions.length
    case "Projects":
      return results.projects.length + results.closed.length
    case "Messages":
      return messages.length
    default:
      return null
  }
}

// nextTab walks the filter row and wraps at both ends, so Tab alone reaches
// every tab and Shift+Tab is never a dead key on the first one.
export function nextTab(tab: PaletteTab, step: number): PaletteTab {
  const count = PALETTE_TABS.length
  const index = (PALETTE_TABS.indexOf(tab) + step + count) % count
  return PALETTE_TABS[index] ?? tab
}
