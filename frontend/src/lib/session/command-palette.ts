// The command palette's data join: open projects and their sessions become a
// flat, filterable list the palette lists and routes to. Kept pure (no React,
// no stores) so the flatten and filter are testable without a render.

import type { ClosedSession, Project, TranscriptMatch } from "@/lib/api-types"
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

// PaletteHistory is one parked session as the History tab lists it: the store's
// row (`matchedConversation` and its `snippet` saying when and where the
// conversation is what matched) plus the branch its checkout is on *now*, read from git. That is not the
// row's `parkedBranch`, which is the snapshot the close recorded for the search
// to match — a branch moves inside a checkout, so only git can say what the row
// shows. `branch` is "" for a checkout that is gone, which is also the row that
// cannot be resumed, so one absence explains both.
export interface PaletteHistory extends ClosedSession {
  branch: string
  /** The checkout is no longer on disk: this row forgets rather than resumes. */
  gone: boolean
}

// historyRows joins the parked rows to what git says about their checkouts. A
// path with no branch and a path in `missing` are different failures — not a
// repository, versus not there at all — but the row only acts on the second,
// so only that one marks it.
export function historyRows(
  closed: readonly ClosedSession[],
  branches: Readonly<Record<string, string>>,
  missing: ReadonlySet<string>,
): PaletteHistory[] {
  return closed.map((session) => ({
    ...session,
    branch: branches[session.path] ?? "",
    gone: missing.has(session.path),
  }))
}

export interface PaletteResults {
  sessions: PaletteSession[]
  projects: Project[]
  // One page of the closed projects the store matched, which the reopen menu
  // shows five of: past that the palette is the only way back to one.
  closed: Project[]
  // Every closed project the term matched, so the group header can report a
  // page that was cut instead of cutting it in silence.
  closedTotal: number
  // The parked sessions — what the History tab lists, and the only group whose
  // rows are not in the workspace at all.
  history: PaletteHistory[]
}

export function filterPalette(
  query: string,
  allSessions: readonly PaletteSession[],
  projects: readonly Project[],
  closed: readonly Project[] = [],
  history: readonly PaletteHistory[] = [],
  closedTotal = 0,
): PaletteResults {
  return {
    closedTotal,
    sessions: allSessions.filter((s) =>
      matchesQuery(`${s.label} ${s.projectName} ${s.path}`, query),
    ),
    projects: projects.filter((p) => matchesQuery(`${p.name} ${p.path}`, query)),
    closed: closed.filter((p) => matchesQuery(`${p.name} ${p.path}`, query)),
    // Both branches are in the haystack because they can disagree: the store
    // matched the one it recorded at the close, and the row shows the one git
    // reads now. Filtering on either alone would drop a row the other half of
    // the search kept.
    // A row the store matched on its conversation is kept without asking again:
    // the snippet is one window onto a hit that may be a megabyte of prose — or
    // no window at all, for a hit the index found by folding an accent — so
    // re-testing the query against the row would drop the answer the search
    // just gave.
    history: history.filter(
      (h) =>
        h.matchedConversation ||
        matchesQuery(`${h.label} ${h.projectName} ${h.branch} ${h.parkedBranch} ${h.path}`, query),
    ),
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
// five kinds of thing at once, which is a page of rows nobody reads: All shows
// the first few of the three worth interrupting for, and a tab lists one kind
// whole — including the closed projects and the closed sessions, which only
// their own tabs carry.
//
// History is last because it is the one posture the palette is not opened in:
// the other four are asked mid-work, and this one is asked when the work is
// already over and somebody is looking for what they did.
export const PALETTE_TABS = ["All", "Sessions", "Projects", "Messages", "History"] as const

export type PaletteTab = (typeof PALETTE_TABS)[number]

// ALL_TAB_ROWS is how much of a group the All tab shows. Enough that a project
// with a handful of sessions is not represented by its first two, and short
// enough that the three groups still fit without scrolling.
const ALL_TAB_ROWS = 5

// rankSessions puts the sessions worth jumping to at the top of the list the
// All tab then cuts to ALL_TAB_ROWS. Order alone is what decides which sessions
// survive that cut, and the natural one — the tab strip's, Home pinned first —
// hands the whole group to whatever sits in the first project: a shell parked
// on a TUI is what somebody keeps in Home, and never what Ctrl+K is opened for.
//
// running is a snapshot of the sessions holding a turn (busy or blocked on a
// prompt), taken when the palette opens; a shell is last because it has no turn
// to hold and no agent to come back to.
export function rankSessions(
  sessions: readonly PaletteSession[],
  running: ReadonlySet<string>,
): PaletteSession[] {
  const rank = (session: PaletteSession): number =>
    running.has(session.sessionId) ? 0 : session.kind === "shell" ? 2 : 1
  // Sort is stable, so sessions of equal rank keep the order the tabs put them
  // in — the ranking reorders what the user wants first, never everything.
  return [...sessions].sort((a, b) => rank(a) - rank(b))
}

// PaletteRow is one option in the list, carrying what running it needs. The
// groups are rendered in order and the rows flattened back out for the arrow
// keys, so a row's index is its position in that flat list and nothing else.
export type PaletteRow =
  | { kind: "session"; session: PaletteSession }
  | { kind: "project"; project: Project }
  | { kind: "closed"; project: Project }
  | { kind: "message"; message: PaletteMessage }
  | { kind: "history"; session: PaletteHistory }

export interface PaletteGroup {
  label: string
  rows: PaletteRow[]
  // Rows the group holds before the All tab's cut, so its header can say what
  // it is leaving out. Equal to rows.length when nothing was cut.
  total: number
  // What the header says instead of that page count, when there is something
  // more urgent to say than which slice of a match is on screen.
  note?: string
}

// rowKey identifies a row inside the list it is rendered in. A session and a
// message about it share an id, and they never share a group.
export function rowKey(row: PaletteRow): string {
  switch (row.kind) {
    case "session":
      return row.session.sessionId
    case "message":
      return row.message.sessionId
    case "history":
      return row.session.id
    default:
      return row.project.id
  }
}

// INDEX_CAP_LABEL is terminal.indexTextBytes spelled the way a reader reads it,
// and it is spelled here rather than sent with the row: the backend answers
// whether the cap cut, which is the fact, and how big the cap is belongs to the
// sentence the window writes. `command-palette.test.ts` pins the string, so the
// two move together the way every other hand-owned mirror in this file does.
export const INDEX_CAP_LABEL = "8 MB"

// historyIndexNote is the line a parked session shows when its conversation was
// indexed only in part: the cap kept the newest of it and dropped the oldest, so
// a search that found nothing in this session found nothing in the part that was
// kept. Undefined for every row indexed whole, which is nearly all of them.
export function historyIndexNote(row: PaletteHistory): string | undefined {
  return row.truncated ? `indexed: newest ${INDEX_CAP_LABEL}` : undefined
}

// historyAction is what Enter does with one history row, and what its hint bar
// says. A checkout that is gone has nothing to resume into — the row is a
// leftover PurgeWorktreeSessions never collected, because the removal never went
// through lich — so the only honest action left is to drop it.
export function historyAction(row: PaletteHistory): "resume" | "forget" {
  return row.gone ? "forget" : "resume"
}

// cap of 0 means no cap: the tab that names one kind lists all of it. `total`
// is how many rows the group stands for, which is the caller's to say when the
// backend cut the list before it got here; never fewer than the rows on screen.
function group(label: string, rows: PaletteRow[], cap: number, total = rows.length): PaletteGroup {
  return { label, rows: cap > 0 ? rows.slice(0, cap) : rows, total: Math.max(total, rows.length) }
}

// historyTotal is how many parked sessions matched the query in the store, which
// is more than `results.history` holds whenever the store's page cut it. It
// defaults to 0 for the callers that do not ask the store at all — the tests and
// the tabs that list nothing parked — and the group falls back to its own rows.
//
// indexing is how many parked sessions the store has not read a conversation out
// of yet, which is the other way the History list can be short, and the one the
// user can do nothing about but wait, so the header says it.
export function paletteGroups(
  tab: PaletteTab,
  results: PaletteResults,
  messages: readonly PaletteMessage[],
  historyTotal = 0,
  indexing = 0,
): PaletteGroup[] {
  const sessions = results.sessions.map((session): PaletteRow => ({ kind: "session", session }))
  const open = results.projects.map((project): PaletteRow => ({ kind: "project", project }))
  const closed = results.closed.map((project): PaletteRow => ({ kind: "closed", project }))
  const said = messages.map((message): PaletteRow => ({ kind: "message", message }))
  const history = results.history.map((session): PaletteRow => ({ kind: "history", session }))

  const groups = ((): PaletteGroup[] => {
    switch (tab) {
      case "Sessions":
        return [group("Sessions", sessions, 0)]
      case "Projects":
        return [group("Open", open, 0), group("Closed", closed, 0, results.closedTotal)]
      case "Messages":
        return [group("Messages", said, 0)]
      case "History":
        // The cut this header reports happened in the store, not in the slice
        // above: the query matched more parked sessions than one page carries.
        // A backfill still reading parked conversations displaces it: a list
        // that cannot see every session yet is worth saying before a page
        // boundary is.
        return [
          {
            ...group("Closed sessions", history, 0),
            total: Math.max(historyTotal, history.length),
            note:
              indexing > 0 ? `indexing ${indexing} session${indexing === 1 ? "" : "s"}` : undefined,
          },
        ]
      // No closed projects or closed sessions here: bringing one back is not
      // what the palette is reached for mid-work, and the rows it costs are
      // rows the sessions and the open projects are cut to make room for. Their
      // own tabs list them whole, which is where somebody looking already goes.
      default:
        return [
          group("Sessions", sessions, ALL_TAB_ROWS),
          group("Projects", open, ALL_TAB_ROWS),
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
    case "History":
      return results.history.length
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
