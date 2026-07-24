// Session state model for multi-session-per-project. A project owns an ordered
// list of terminal sessions and tracks which one is active. Sessions decouple
// from the project: each has its own opaque id used as the backend PTY key, so
// the terminal service needs no knowledge of projects at all.
//
// Every function here is pure — it returns a new state and never mutates the
// input — which keeps the reducer logic testable without React or a PTY.

import { applyOrder } from "./reorder"

// Provider ids that can back a session, mirrored from internal/providers.Registry
// (Go) — keep in sync. A session's kind is one of these or the plain shell.
export const PROVIDER_KINDS = ["claude", "codex", "opencode", "crush"] as const
export type ProviderKind = (typeof PROVIDER_KINDS)[number]

// What a session's PTY runs: a provider's CLI or the user's shell. Values match
// the backend (store column + terminal.Start).
export type SessionKind = ProviderKind | "shell"

// isSessionKind narrows a persisted or otherwise untrusted string to a
// SessionKind, so hydration can fall back on an unrecognized value instead of
// carrying a bad kind into state.
export function isSessionKind(value: string): value is SessionKind {
  return value === "shell" || (PROVIDER_KINDS as readonly string[]).includes(value)
}

export interface Session {
  id: string
  label: string
  kind: SessionKind
  // Working directory when the session lives in a git worktree; absent means
  // the project's own path.
  path?: string
  // The provider conversation this card ran before the last restart, reported
  // by the provider's session-start hook and read back on hydration. Only ever
  // set by the store: a session created in this run has none, and the hook's
  // later report is not mirrored here — a running session has nothing to resume.
  providerSessionId?: string
}

export interface ProjectSessions {
  sessions: Session[]
  activeId: string
  // Monotonic per-project counter for default labels. It only grows, so closing
  // a session never renumbers the survivors.
  nextSeq: number
}

export type SessionState = Record<string, ProjectSessions>

export function createProjectSessions(
  firstSessionId: string,
  kind: SessionKind = "claude",
): ProjectSessions {
  return {
    sessions: [{ id: firstSessionId, label: "Session 1", kind }],
    activeId: firstSessionId,
    nextSeq: 2,
  }
}

// addSession appends a session to a project and makes it active. If the project
// has no entry yet, it is created with this session as the first. A worktree
// session carries its own path and is labeled after the worktree instead of the
// sequential default.
export function addSession(
  state: SessionState,
  projectId: string,
  sessionId: string,
  kind: SessionKind = "claude",
  path = "",
  label?: string,
): SessionState {
  const current = state[projectId]
  if (!current) {
    return { ...state, [projectId]: createProjectSessions(sessionId, kind) }
  }
  const session: Session = {
    id: sessionId,
    label: label || `Session ${current.nextSeq}`,
    kind,
    ...(path ? { path } : {}),
  }
  return {
    ...state,
    [projectId]: {
      sessions: [...current.sessions, session],
      activeId: sessionId,
      nextSeq: current.nextSeq + 1,
    },
  }
}

// closeSession removes a session. When the active one is closed, focus moves to
// a neighbor. The project entry is kept even when empty so nextSeq survives: a
// project emptied and then reopened keeps counting labels up.
export function closeSession(
  state: SessionState,
  projectId: string,
  sessionId: string,
): SessionState {
  const current = state[projectId]
  if (!current) {
    return state
  }
  const index = current.sessions.findIndex((s) => s.id === sessionId)
  if (index === -1) {
    return state
  }
  const sessions = current.sessions.filter((s) => s.id !== sessionId)
  const activeId =
    current.activeId === sessionId ? neighborId(sessions, index) : current.activeId
  return { ...state, [projectId]: { ...current, sessions, activeId } }
}

// neighborId picks the session that fills the closed slot, falling back to the
// previous one, or "" when the list is now empty.
function neighborId(sessions: Session[], removedIndex: number): string {
  if (sessions.length === 0) {
    return ""
  }
  const next = sessions[removedIndex] ?? sessions[removedIndex - 1]
  return next.id
}

// restoreSession re-adds a parked session — its own id, label and
// providerSessionId intact — to a project and focuses it, without advancing the
// label counter: a resume brings back an existing session, it does not mint a
// new numbered one. An id already present is just focused; an unknown project is
// ignored.
export function restoreSession(
  state: SessionState,
  projectId: string,
  session: Session,
): SessionState {
  const current = state[projectId]
  if (!current) {
    return state
  }
  if (current.sessions.some((s) => s.id === session.id)) {
    return setActiveSession(state, projectId, session.id)
  }
  return {
    ...state,
    [projectId]: {
      ...current,
      sessions: [...current.sessions, session],
      activeId: session.id,
    },
  }
}

// setActiveSession focuses an existing session; unknown ids are ignored.
export function setActiveSession(
  state: SessionState,
  projectId: string,
  sessionId: string,
): SessionState {
  const current = state[projectId]
  if (!current || !current.sessions.some((s) => s.id === sessionId)) {
    return state
  }
  return { ...state, [projectId]: { ...current, activeId: sessionId } }
}

// renameSession sets a session's label. Unknown project or session ids are
// ignored, returning the input state unchanged.
export function renameSession(
  state: SessionState,
  projectId: string,
  sessionId: string,
  label: string,
): SessionState {
  const current = state[projectId]
  if (!current || !current.sessions.some((s) => s.id === sessionId)) {
    return state
  }
  return {
    ...state,
    [projectId]: {
      ...current,
      sessions: current.sessions.map((s) =>
        s.id === sessionId ? { ...s, label } : s,
      ),
    },
  }
}

// reorderSessions rearranges a project's sessions to match the given id order,
// leaving the active session and the label counter alone — a drag only moves
// cards. An id list that no longer names the project's exact session set (a
// close raced the drag) is dropped, returning the input state unchanged.
export function reorderSessions(
  state: SessionState,
  projectId: string,
  ids: string[],
): SessionState {
  const current = state[projectId]
  if (!current) {
    return state
  }
  const sessions = applyOrder(current.sessions, ids)
  if (!sessions) {
    return state
  }
  return { ...state, [projectId]: { ...current, sessions } }
}

export function removeProject(
  state: SessionState,
  projectId: string,
): SessionState {
  if (!(projectId in state)) {
    return state
  }
  const next = { ...state }
  delete next[projectId]
  return next
}

// resumableSession returns the session whose PTY should ask before it spawns,
// because it carries the provider conversation it ran before the last restart.
// Null for everything with nothing to resume: unknown ids, sessions created in
// this run, providers with no resume flag wired (only Claude Code has one), and
// shell sessions — whose shell cannot reopen a conversation even when a hand-run
// provider CLI left an id on their row.
export function resumableSession(
  state: SessionState,
  projectId: string,
  sessionId: string,
): Session | null {
  const session = state[projectId]?.sessions.find((s) => s.id === sessionId)
  if (!session || session.kind !== "claude" || !session.providerSessionId) {
    return null
  }
  return session
}

export function sessionsOf(state: SessionState, projectId: string): Session[] {
  return state[projectId]?.sessions ?? []
}

// A worktree's sessions under one roof. `path` is the checkout root ("" for the
// project's own directory); `sessions` keeps the group's flat relative order.
export interface SessionGroup {
  path: string
  sessions: Session[]
}

// groupByWorktree buckets sessions by their static checkout path (session.path;
// "" = the project root), keeping first-appearance order for the groups and flat
// order within each. It keys off the spawn-time path, never a live cwd, so a `cd`
// deeper into a checkout never moves a card to another group.
export function groupByWorktree(sessions: Session[]): SessionGroup[] {
  const groups: SessionGroup[] = []
  const byPath = new Map<string, SessionGroup>()
  for (const session of sessions) {
    const path = session.path ?? ""
    let group = byPath.get(path)
    if (!group) {
      group = { path, sessions: [] }
      byPath.set(path, group)
      groups.push(group)
    }
    group.sessions.push(session)
  }
  return groups
}

// True only for the last session in a worktree checkout. Removing a checkout a
// sibling session still occupies would throw away its work, so only the last
// occupant gets offered the keep/remove prompt.
export function isLastWorktreeSession(
  sessions: Session[],
  session: Session,
): boolean {
  if (!session.path) {
    return false
  }
  return !sessions.some((s) => s.id !== session.id && s.path === session.path)
}

export function activeSessionId(state: SessionState, projectId: string): string {
  return state[projectId]?.activeId ?? ""
}

// projectOfSession returns the id of the project owning a session, or "" when no
// project holds it. Backend events (e.g. the auto ai-title) carry only a session
// id, so the provider uses this to locate the project the reducer needs.
export function projectOfSession(state: SessionState, sessionId: string): string {
  for (const [projectId, project] of Object.entries(state)) {
    if (project.sessions.some((s) => s.id === sessionId)) {
      return projectId
    }
  }
  return ""
}
