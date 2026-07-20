// The frontend half of the session event contract (docs/hooks/): the names the
// backend emits under, the narrowing of their untrusted payloads, and the rules
// driven off them. The names are duplicated from internal/terminal/terminal.go —
// unavoidable across the Go/TS boundary, so at least the TS side lives here
// alone. Kept pure (no bindings, no React) so the provider and the card only
// wire it up.

import {projectOfSession, type SessionState} from "./sessions"

// Global event carrying a session's Claude Code processing state (see
// terminal.statusEventName). Payload: { id, state }. Global rather than
// per-session so one subscription taken at load covers every session: a card is
// only mounted while its project is active, and a name suffixed with the session
// id can only reach a subscriber that exists when it is emitted.
export const STATUS_EVENT = "session-status"

// Global event the backend emits when it auto-applies a session's ai-title as
// its label (see terminal.titleEventName). Payload: { id, label }.
export const TITLE_EVENT = "session-title"

// Global event the backend emits when a session likely changed files on disk
// (see terminal.touchedEventName). Payload: { id }.
export const TOUCHED_EVENT = "session-touched"

// Global event the backend emits with a session's live working directory (see
// terminal.cwdEventName): once with the directory the PTY starts in, then on
// every change the cwd watcher observes. Payload: { id, cwd }.
export const CWD_EVENT = "session-cwd"

// The states a card renders an indicator for. The contract also defines "idle"
// (SessionEnd), which maps to no indicator like any unknown value does.
const RENDERED_STATUSES = ["busy", "done", "waiting"] as const

export type SessionStatus = (typeof RENDERED_STATUSES)[number]

// toSessionStatus narrows a status payload to the states the card renders. The
// payload crosses a process boundary, so anything else — the contract's "idle",
// a state from a newer plugin, a malformed value — yields null, which clears the
// indicator rather than stranding a stale one.
export function toSessionStatus(data: unknown): SessionStatus | null {
  if (typeof data !== "string") {
    return null
  }
  return (RENDERED_STATUSES as readonly string[]).includes(data)
    ? (data as SessionStatus)
    : null
}

// session-touched carries only a session id.
export function isIdEvent(data: unknown): data is {id: string} {
  return (
    typeof data === "object" &&
    data !== null &&
    typeof (data as {id?: unknown}).id === "string"
  )
}

export function isStatusEvent(data: unknown): data is {id: string; state: string} {
  return isIdEvent(data) && typeof (data as {state?: unknown}).state === "string"
}

export function isTitleEvent(data: unknown): data is {id: string; label: string} {
  return isIdEvent(data) && typeof (data as {label?: unknown}).label === "string"
}

export function isCwdEvent(data: unknown): data is {id: string; cwd: string} {
  return isIdEvent(data) && typeof (data as {cwd?: unknown}).cwd === "string"
}

// shouldToastAttention decides whether a session needing the user deserves the
// global toast. The session already in focus (active session of the active
// project) does not: its own terminal shows the prompt. Every other session
// does, including one in a background project whose card is not even mounted.
// An unknown session has nowhere to route, so it stays silent.
export function shouldToastAttention(
  state: SessionState,
  sessionId: string,
  activeProjectId: string | undefined,
): boolean {
  const projectId = projectOfSession(state, sessionId)
  const project = state[projectId]
  if (!project) {
    return false
  }
  const focused = projectId === activeProjectId && project.activeId === sessionId
  return !focused
}
