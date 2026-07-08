// Session state model for multi-session-per-project. A project owns an ordered
// list of terminal sessions and tracks which one is active. Sessions decouple
// from the project: each has its own opaque id used as the backend PTY key, so
// the terminal service needs no knowledge of projects at all.
//
// Every function here is pure — it returns a new state and never mutates the
// input — which keeps the reducer logic testable without React or a PTY.

export interface Session {
  id: string
  label: string
}

export interface ProjectSessions {
  sessions: Session[]
  activeId: string
  // Monotonic per-project counter for default labels. It only grows, so closing
  // a session never renumbers the survivors.
  nextSeq: number
}

export type SessionState = Record<string, ProjectSessions>

// createProjectSessions seeds a project with its first session.
export function createProjectSessions(firstSessionId: string): ProjectSessions {
  return {
    sessions: [{ id: firstSessionId, label: "Session 1" }],
    activeId: firstSessionId,
    nextSeq: 2,
  }
}

// addSession appends a session to a project and makes it active. If the project
// has no entry yet, it is created with this session as the first.
export function addSession(
  state: SessionState,
  projectId: string,
  sessionId: string,
): SessionState {
  const current = state[projectId]
  if (!current) {
    return { ...state, [projectId]: createProjectSessions(sessionId) }
  }
  const session: Session = { id: sessionId, label: `Session ${current.nextSeq}` }
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
// a neighbor. The project entry is kept even when empty so nextSeq (and the
// caller's recreate-on-empty policy) survive.
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

// removeProject drops a project and all its sessions.
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

export function sessionsOf(state: SessionState, projectId: string): Session[] {
  return state[projectId]?.sessions ?? []
}

export function activeSessionId(state: SessionState, projectId: string): string {
  return state[projectId]?.activeId ?? ""
}
