// The frontend half of the session event contract (docs/hooks/): the names the
// backend emits under, the narrowing of their untrusted payloads, and the rules
// driven off them. The names are duplicated from internal/terminal/terminal.go —
// unavoidable across the Go/TS boundary, so at least the TS side lives here
// alone. Kept pure (no bindings, no React) so the provider and the card only
// wire it up.

import { isSessionKind, projectOfSession, type SessionKind, type SessionState } from "./sessions"

// A subscription to one of the global session events, injected so every store
// below is testable without standing up the /events socket. Returns its
// unsubscribe. One type for all of them: the channel is the same, and a store
// naming its own alias said nothing the event name it subscribes to did not.
export type SessionEventSource = (handler: (data: unknown) => void) => () => void

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

// Global event the backend emits when a provider CLI starts inside a session's
// PTY (see terminal.agentEventName) — a hand-run `claude` in a shell session.
// Payload: { id, agent }; an empty agent (every PTY spawn) clears the mark.
export const AGENT_EVENT = "session-agent"

// Global event the backend emits after a turn ends with the session's
// context-window usage (see terminal.usageEventName). Payload: { id, percent,
// tokens, window, model, effort } — percent is 0–100 of the window, tokens the
// raw input-side count, window the model's context size, model its id, effort
// the reasoning level ("" when the turn records none) — plus an optional
// costUsd, which is present only when the cost readout is on and every model in
// the session has a known price. Its absence is the answer, not a zero.
export const USAGE_EVENT = "session-usage"

// Global event the backend emits while one session has a request open with
// another (see relay.RelayEventName). Payload: { id, peer, direction } — the
// session whose card changes, the label at the other end, and which way the
// request runs. An empty direction clears it: the request is over, answered or
// expired.
export const RELAY_EVENT = "session-relay"

// Global event the backend emits when a session was opened outside the window —
// an agent running `lich open` or its MCP tool (see spawn.OpenedEventName).
// Payload: the whole session, because the card is drawn from it rather than
// looked up: the row is already written and its PTY is already running.
export const OPENED_EVENT = "session-opened"

// Global event the backend emits when a session was closed outside the window —
// an agent running `lich close` or its MCP tool (see spawn.ClosedEventName).
// Payload: { id, projectId, activeId } — the session that left, its project,
// and which card is active now, decided by the row that is already written.
export const CLOSED_EVENT = "session-closed"

// Global event the backend emits when a request's target ended its turn without
// answering through lich (see relay.StalledEventName). Payload:
// { id, targetId, target } — who asked ("" when it was the command line rather
// than a session), and the session holding whatever was produced.
export const RELAY_STALLED_EVENT = "session-relay-stalled"

// One request whose answer, if there is one, is only in the target's terminal.
export interface RelayStalled {
  id: string
  targetId: string
  target: string
}

export function isRelayStalledEvent(data: unknown): data is RelayStalled {
  if (!isIdEvent(data)) {
    return false
  }
  const { targetId, target } = data as { targetId?: unknown; target?: unknown }
  return typeof targetId === "string" && targetId !== "" && typeof target === "string"
}

// Which way an open request runs, from the marked card's side: "out" is waiting
// on someone, "in" is owing someone an answer.
const RELAY_DIRECTIONS = ["out", "in"] as const

export type RelayDirection = (typeof RELAY_DIRECTIONS)[number]

// One session's open request. peer is the label at the other end, empty when
// that end is not a session at all — the `lich` command run from a script or a
// plain shell, which the card names in its own words.
export interface SessionRelay {
  peer: string
  direction: RelayDirection
}

// toSessionRelay narrows a relay payload to an open request, or null when there
// is none: the clearing empty direction, and equally a direction from a newer
// backend that this build cannot draw. Both mean "show nothing", which is
// safer than stranding a mark no event will ever take down.
export function toSessionRelay(data: unknown): SessionRelay | null {
  const { peer, direction } = (data ?? {}) as { peer?: unknown; direction?: unknown }
  if (typeof direction !== "string") {
    return null
  }
  if (!(RELAY_DIRECTIONS as readonly string[]).includes(direction)) {
    return null
  }
  return { peer: typeof peer === "string" ? peer : "", direction: direction as RelayDirection }
}

// A session's context-window occupancy as the footer shows it, and what the
// session has cost so far. costUsd is null whenever the backend sent no number:
// the setting is off (the case on a subscription, where the figure is noise) or
// a model has no price yet. Nothing is rendered for it then.
export interface SessionUsage {
  percent: number
  tokens: number
  window: number
  model: string
  effort: string
  costUsd: number | null
}

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
  return (RENDERED_STATUSES as readonly string[]).includes(data) ? (data as SessionStatus) : null
}

// Global event the backend emits when a session's relay inbox changes size —
// results waiting to be collected (see relay.InboxEventName). Payload:
// { id, count }; zero clears the mark.
export const INBOX_EVENT = "session-inbox"

// toInboxCount narrows an inbox payload to a drawable count. Anything that is
// not a positive number — a malformed payload, a shape from another build —
// reads as zero, which clears the mark rather than stranding a stale one.
export function toInboxCount(data: unknown): number {
  const { count } = (data ?? {}) as { count?: unknown }
  if (typeof count !== "number" || !Number.isFinite(count) || count <= 0) {
    return 0
  }
  return Math.floor(count)
}

// session-touched carries only a session id.
export function isIdEvent(data: unknown): data is { id: string } {
  return (
    typeof data === "object" && data !== null && typeof (data as { id?: unknown }).id === "string"
  )
}

export function isStatusEvent(data: unknown): data is { id: string; state: string } {
  return isIdEvent(data) && typeof (data as { state?: unknown }).state === "string"
}

// isIdleEvent reports the status the contract calls SessionEnd — the provider
// CLI leaving the PTY. Three stores clear a live mark on it (the agent, the
// relay request, the inbox count), and each of them was spelling the narrowing
// out for itself, two through a cast that stepped around isStatusEvent above.
export function isIdleEvent(data: unknown): data is { id: string; state: "idle" } {
  return isStatusEvent(data) && data.state === "idle"
}

// The tool a session's turn is running: the harness's own name for it, and its
// own words for what it acts on ("" when it offered none). Never translated
// between providers: the word on the card is the word in the terminal beside
// it (the vocabularies are tabled in docs/hooks/session-state.md).
export interface SessionTool {
  name: string
  detail: string
}

// statusTool reads the tool pair off a status payload, or null when the report
// names no tool. Whether a report *clears* the line is the store's call, not
// this one's: a "busy" naming nothing is the gap between two tools, not the end
// of the turn (see session-tool-store).
export function statusTool(data: unknown): SessionTool | null {
  const { tool, detail } = (data ?? {}) as { tool?: unknown; detail?: unknown }
  if (typeof tool !== "string" || tool === "") {
    return null
  }
  return { name: tool, detail: typeof detail === "string" ? detail : "" }
}

// A session opened outside the window, as the workspace needs it: the card to
// draw and the project's label counter after that card took its number.
export interface OpenedSession {
  id: string
  projectId: string
  label: string
  kind: SessionKind
  path: string
  nextSeq: number
  // The session that asked for this one, and what it was called at the time.
  // Both "" when the opener was not a session — `lich open` from a plain shell.
  originSessionId: string
  originLabel: string
}

// toOpenedSession narrows a session-opened payload, or null when it names
// nothing this window can place: no project id, no label, or a kind this build
// does not know (a provider added by a newer backend). Dropping it costs the
// card until the next load, which reads the row that is already written —
// adopting a session whose kind cannot spawn would cost more.
export function toOpenedSession(data: unknown): OpenedSession | null {
  if (!isIdEvent(data)) {
    return null
  }
  const { projectId, label, kind, path, nextSeq, originSessionId, originLabel } = data as {
    projectId?: unknown
    label?: unknown
    kind?: unknown
    path?: unknown
    nextSeq?: unknown
    originSessionId?: unknown
    originLabel?: unknown
  }
  if (typeof projectId !== "string" || projectId === "" || typeof label !== "string") {
    return null
  }
  if (typeof kind !== "string" || !isSessionKind(kind)) {
    return null
  }
  return {
    id: data.id,
    projectId,
    label,
    kind,
    path: typeof path === "string" ? path : "",
    nextSeq: typeof nextSeq === "number" ? nextSeq : 1,
    originSessionId: typeof originSessionId === "string" ? originSessionId : "",
    originLabel: typeof originLabel === "string" ? originLabel : "",
  }
}

// A session closed outside the window: which card goes, and which one the
// project falls back to.
export interface ClosedSession {
  id: string
  projectId: string
  activeId: string
}

export function toClosedSession(data: unknown): ClosedSession | null {
  if (!isIdEvent(data)) {
    return null
  }
  const { projectId, activeId } = data as { projectId?: unknown; activeId?: unknown }
  if (typeof projectId !== "string" || projectId === "") {
    return null
  }
  return { id: data.id, projectId, activeId: typeof activeId === "string" ? activeId : "" }
}

export function isTitleEvent(data: unknown): data is { id: string; label: string } {
  return isIdEvent(data) && typeof (data as { label?: unknown }).label === "string"
}

export function isCwdEvent(data: unknown): data is { id: string; cwd: string } {
  return isIdEvent(data) && typeof (data as { cwd?: unknown }).cwd === "string"
}

export function isAgentEvent(data: unknown): data is { id: string; agent: string } {
  return isIdEvent(data) && typeof (data as { agent?: unknown }).agent === "string"
}

export function isUsageEvent(data: unknown): data is {
  id: string
  percent: number
  tokens: number
  window: number
  model: string
  effort: string
  costUsd?: number
} {
  return (
    isIdEvent(data) &&
    typeof (data as { percent?: unknown }).percent === "number" &&
    typeof (data as { tokens?: unknown }).tokens === "number" &&
    typeof (data as { window?: unknown }).window === "number" &&
    typeof (data as { model?: unknown }).model === "string" &&
    typeof (data as { effort?: unknown }).effort === "string"
  )
}

// usageCost reads the optional cost off a usage payload. Absent is the common
// case (the readout is off for everyone who is not billed per token), and
// anything that is not a finite number is treated as absent rather than shown:
// a total is either trustworthy or not rendered.
export function usageCost(data: { costUsd?: unknown }): number | null {
  return typeof data.costUsd === "number" && Number.isFinite(data.costUsd) ? data.costUsd : null
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

/** What the desktop channel does with a session that needs the user. */
export type DesktopNotice = "none" | "ask" | "notify"

// decideDesktopNotice is the desktop half of the same report shouldToastAttention
// answers, and it asks a different question: not which session the user is on,
// but whether they are at the window at all. With lich in front there is nothing
// to do — the card's bell and the toast are already in view. With the window
// behind something else every waiting session is unseen, the active one
// included, and the notification is the only channel left. Until the opt-in has
// been answered (null) the first such report puts the question instead of the
// notification.
export function decideDesktopNotice(
  windowFocused: boolean,
  enabled: boolean | null,
): DesktopNotice {
  if (windowFocused) {
    return "none"
  }
  if (enabled === null) {
    return "ask"
  }
  return enabled ? "notify" : "none"
}

/** The two desktop channels' opt-ins, as decideStatusNotice reads them. */
export interface DesktopNotifyPrefs {
  /** A session blocked on the user — the channel the opt-in dialog asks about,
   * hence null while it has not been answered. */
  attention: boolean | null
  /** A turn that finished. A plain switch in Settings, off until it is turned
   * on: never inherited from the answer given to the dialog above. */
  finishedTurn: boolean
}

// decideStatusNotice routes a status report to the desktop channel that owns it.
// Each channel has its own opt-in and both answer to the same focus rule, so
// both go through decideDesktopNotice. Only "waiting" can come back "ask": the
// finished-turn preference is a boolean, so turning it on is itself the answer
// and there is no second question to put.
//
// A "done" repeated with no run in between is dropped: it is the same finished
// turn reported again, not a new one to announce. That collapse lives here, off
// the caller's previous status, rather than in the status store the toast path
// deliberately bypasses — "waiting" must keep notifying on every report, because
// a second permission prompt is a second thing to answer.
export function decideStatusNotice(
  status: SessionStatus | null,
  previous: SessionStatus | null,
  windowFocused: boolean,
  prefs: DesktopNotifyPrefs,
): DesktopNotice {
  if (status === "waiting") {
    return decideDesktopNotice(windowFocused, prefs.attention)
  }
  if (status === "done" && previous !== "done") {
    return decideDesktopNotice(windowFocused, prefs.finishedTurn)
  }
  return "none"
}
