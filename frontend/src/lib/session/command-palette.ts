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
}

export function filterPalette(
  query: string,
  allSessions: readonly PaletteSession[],
  projects: readonly Project[],
): PaletteResults {
  return {
    sessions: allSessions.filter((s) =>
      matchesQuery(`${s.label} ${s.projectName} ${s.path}`, query),
    ),
    projects: projects.filter((p) => matchesQuery(`${p.name} ${p.path}`, query)),
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
