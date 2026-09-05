// Session state model for multi-session-per-project. A project owns an ordered
// list of terminal sessions and tracks which one is active. Sessions decouple
// from the project: each has its own opaque id used as the backend PTY key, so
// the terminal service needs no knowledge of projects at all.
//
// Every function here is pure — it returns a new state and never mutates the
// input — which keeps the reducer logic testable without React or a PTY.

import { applyOrder } from "@/lib/reorder"
import type { PaneGroup } from "./panes"

// Provider ids that can back a session, mirrored from internal/providers.Registry
// (Go) — keep in sync. A session's kind is one of these or the plain shell.
export const PROVIDER_KINDS = [
  "claude",
  "codex",
  "antigravity",
  "opencode",
  "omp",
  "crush",
  "cursor",
  "kiro",
] as const
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
  // The command this terminal opens into, absent for a plain shell and for
  // every provider session — the store refuses to record one against a card
  // whose PTY runs an agent.
  entrypoint?: string
  // Whether this session's PTY runs inside the sandbox (internal/sandbox).
  // Absent means it does not — the spawn decides it, from the provider's rung,
  // the checkout and any per-session override, and reports the verdict back;
  // the card never re-derives it.
  sandboxed?: boolean
  // Kept at the head of the project's list and refused a close until unpinned.
  pinned?: boolean
  // The session that asked for this one, when it was opened by delegation:
  // absent for every session opened from the window. The id is the live half —
  // it resolves to whatever that session is called now — and the label is the
  // name the delegation happened under, which is what is left once the parent
  // is closed. Read them through sessionOrigin, never on their own.
  originSessionId?: string
  originLabel?: string
}

export interface ProjectSessions {
  sessions: Session[]
  activeId: string
  // Monotonic per-project counter for default labels. It only grows, so closing
  // a session never renumbers the survivors.
  nextSeq: number
}

export type SessionState = Record<string, ProjectSessions>

// What a project with no entry yet counts as. Seeding the missing entry rather
// than branching on it is what keeps the first session of a project from being
// built by a second, poorer code path: the one that used to run here dropped
// the worktree path and the label it was handed, so a worktree session opened
// as a project's first landed in the project root while the store recorded the
// checkout.
const NO_SESSIONS: ProjectSessions = { sessions: [], activeId: "", nextSeq: 1 }

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
  const current = state[projectId] ?? NO_SESSIONS
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

// adoptSession takes in a session the backend already created and persisted —
// one an agent opened through the `lich` CLI or its MCP tools — and appends its
// card, carrying the project's label counter forward to the value the row was
// written with.
//
// It deliberately does not focus what it adds, which is the whole difference
// from addSession: nobody in front of the window asked for this session, and a
// meta-agent opening three of them would otherwise drag the view along three
// times while the user is working. An unknown project is ignored — its cards are
// not on screen to add to, and the next load reads the row anyway.
export function adoptSession(
  state: SessionState,
  projectId: string,
  session: Session,
  nextSeq: number,
): SessionState {
  const current = state[projectId]
  if (!current || current.sessions.some((s) => s.id === session.id)) {
    return state
  }
  return {
    ...state,
    [projectId]: {
      ...current,
      sessions: [...current.sessions, session],
      nextSeq: Math.max(current.nextSeq, nextSeq),
    },
  }
}

// dropClosedSession removes a session the backend already closed — one an agent
// closed through the CLI or its MCP tools — and focuses whichever card that
// write recorded as the project's active one. Nothing is persisted from here:
// the row is gone, and choosing a different card than the row names would put
// the window and the next launch on different sessions.
export function dropClosedSession(
  state: SessionState,
  projectId: string,
  sessionId: string,
  activeId: string,
): SessionState {
  const current = state[projectId]
  if (!current || !current.sessions.some((s) => s.id === sessionId)) {
    return state
  }
  return {
    ...state,
    [projectId]: {
      ...current,
      sessions: current.sessions.filter((s) => s.id !== sessionId),
      activeId,
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
  const activeId = current.activeId === sessionId ? neighborId(sessions, index) : current.activeId
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

// restoreSession re-adds a session that left the list — parked for a resume, or
// closed and undone — with its own id, label and providerSessionId intact, and
// focuses it, without advancing the label counter: it brings back an existing
// session, it does not mint a new numbered one. It lands at `index` when one is
// given (the slot an undone close left behind) and at the tail otherwise. An id
// already present is just focused; an unknown project is ignored.
export function restoreSession(
  state: SessionState,
  projectId: string,
  session: Session,
  index?: number,
): SessionState {
  const current = state[projectId]
  if (!current) {
    return state
  }
  if (current.sessions.some((s) => s.id === session.id)) {
    return setActiveSession(state, projectId, session.id)
  }
  const sessions = [...current.sessions]
  sessions.splice(index ?? sessions.length, 0, session)
  return { ...state, [projectId]: { ...current, sessions, activeId: session.id } }
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
      sessions: current.sessions.map((s) => (s.id === sessionId ? { ...s, label } : s)),
    },
  }
}

// setSessionSandboxed records whether a session's PTY runs confined, as the
// spawn reported it. Unknown ids leave the state untouched, and a value that
// already matches returns the same object — the event fires on every spawn, and
// a re-render per respawn of an unchanged card is a card that flickers.
export function setSessionSandboxed(
  state: SessionState,
  sessionId: string,
  sandboxed: boolean,
): SessionState {
  const projectId = projectOfSession(state, sessionId)
  const current = projectId ? state[projectId] : undefined
  const session = current?.sessions.find((s) => s.id === sessionId)
  if (!projectId || !current || !session || (session.sandboxed ?? false) === sandboxed) {
    return state
  }
  return {
    ...state,
    [projectId]: {
      ...current,
      sessions: current.sessions.map((s) => {
        if (s.id !== sessionId) {
          return s
        }
        // Dropped rather than set to undefined, so an unconfined session carries
        // no key at all — the shape the two hydration paths produce.
        const { sandboxed: _was, ...rest } = s
        return sandboxed ? { ...rest, sandboxed: true } : rest
      }),
    },
  }
}

// setSessionEntrypoint records the command a terminal session opens into, and
// takes the command as the card's label while that label is still whatever lich
// named it — a card called "Terminal 2" that runs lazygit is a card the user has
// to open to identify. `auto` is what the store answered about the rename it was
// asked for in the same breath, so the decision is never guessed here: a card
// the user named keeps its name, and the command shows in its tooltip instead.
//
// Unknown project or session ids are ignored, returning the input state
// unchanged, and a session that is not a terminal is left alone — the store
// refuses the write for it, so accepting it here would draw a card that does not
// match the row behind it.
export function setSessionEntrypoint(
  state: SessionState,
  projectId: string,
  sessionId: string,
  entrypoint: string,
  auto: boolean,
): SessionState {
  const current = state[projectId]
  const session = current?.sessions.find((s) => s.id === sessionId)
  if (!current || !session || session.kind !== "shell") {
    return state
  }
  return {
    ...state,
    [projectId]: {
      ...current,
      sessions: current.sessions.map((s) =>
        s.id === sessionId
          ? { ...s, entrypoint, ...(auto && entrypoint ? { label: entrypoint } : {}) }
          : s,
      ),
    },
  }
}

// The pinned block's stand-in id, for the same reason ROOT_GROUP_KEY exists: it
// is not a path. The block gathers sessions from every checkout, so it can
// collide with no worktree — those paths are absolute.
export const PINNED_GROUP_KEY = "__pinned__"

// One block of the sidebar: a split group, the pinned sessions, or one
// worktree's.
export interface SidebarGroup {
  key: string
  // True for the pinned block. It is not a checkout — no path of its own, no
  // pull request — and it never moves among the others.
  pinned: boolean
  // The split group this block draws, or null. Its sessions come from any
  // checkout, so it is keyed by the group's own id and can collide with no
  // worktree path. A project has as many of these as the user has made, drawn
  // above everything else in the order they arranged.
  stage: PaneGroup | null
  // The checkout root ("" for the project's own directory), empty for the
  // gathered blocks.
  path: string
  sessions: Session[]
}

// sidebarGroups splits a project's sessions into the blocks the sidebar draws:
// the pinned ones first — that is what a pin promises — then the walls and the
// worktrees interleaved, each block landing where its first card sits in the
// stored list. Each block keeps that stored (drag) order inside, except a
// wall's, which keeps the order of the panes it draws.
//
// One order for both kinds of block, read off the one list, is what lets a drag
// move a wall past a worktree: block order is nowhere else, so moving a block is
// moving its cards inside the flat list and nothing has a second opinion.
//
// Neither a split nor a pin rewrites that list; both only lift a card into a
// block, which is what lets a session dropped from either land back among its
// old neighbours instead of being stranded. The store hands the same order back,
// so a reload draws what the live change did.
//
// A session both pinned and on a wall is drawn in the wall's block: the pin
// promises the top of the list rather than one particular header, and the card
// keeps its own mark either way. A wall whose sessions are all gone draws
// nothing — resolveGroups has already dropped it from what is stored.
export function sidebarGroups(
  sessions: Session[],
  stage: readonly PaneGroup[] = [],
): SidebarGroup[] {
  const byId = new Map(sessions.map((session) => [session.id, session]))
  const wallOf = new Map<string, PaneGroup>()
  for (const group of stage) {
    for (const id of group.cells) {
      if (byId.has(id)) {
        wallOf.set(id, group)
      }
    }
  }
  const blocks: SidebarGroup[] = []
  const pinned = sessions.filter((s) => s.pinned && !wallOf.has(s.id))
  if (pinned.length > 0) {
    blocks.push({ key: PINNED_GROUP_KEY, pinned: true, stage: null, path: "", sessions: pinned })
  }

  const loose = sessions.filter((s) => !s.pinned && !wallOf.has(s.id))
  const byPath = new Map(
    groupByWorktree(loose).map((group) => [groupKey(group.path), group] as const),
  )
  const drawn = new Set<string>()
  for (const session of sessions) {
    const wall = wallOf.get(session.id)
    const key = wall ? wall.id : session.pinned ? "" : groupKey(session.path ?? "")
    if (!key || drawn.has(key)) {
      continue
    }
    drawn.add(key)
    if (wall) {
      blocks.push({
        key,
        pinned: false,
        stage: wall,
        path: "",
        sessions: wall.cells.flatMap((id) => byId.get(id) ?? []),
      })
      continue
    }
    const group = byPath.get(key)
    if (group) {
      blocks.push({ key, pinned: false, stage: null, path: group.path, sessions: group.sessions })
    }
  }
  return blocks
}

// reorderSubset returns the full id order that hands `ids` to the sessions the
// predicate picks and leaves every other session exactly where it is. A drag
// inside one block must not move the blocks around it, and the pinned block is
// not even contiguous in the stored list — it is a filter over it.
//
// An `ids` that does not name that subset exactly yields a list with a repeat
// (or a gap), which reorderSessions rejects wholesale — the same way it rejects
// an order a close has raced.
export function reorderSubset(
  stored: Session[],
  ids: string[],
  member: (session: Session) => boolean,
): string[] {
  const queue = [...ids]
  return stored.map((session) => (member(session) ? (queue.shift() ?? session.id) : session.id))
}

// dragOrder returns the stored id order that lays the sidebar's movable blocks
// out in `keys` — every block but the pinned one, walls and checkouts alike,
// since they share one drag list and one place to be stored.
//
// The slots it may fill are the cards those blocks actually drew, which is not
// "every unpinned session": a pinned card on a wall is drawn in the wall, so its
// slot travels with the wall while the pinned block's own cards stay put. Get
// that set wrong and reorderSessions rejects the whole order — the drop becomes
// a silent no-op — which is why it is derived from the blocks rather than
// restated.
export function dragOrder(stored: Session[], groups: SidebarGroup[], keys: string[]): string[] {
  const movable = groups.filter((group) => !group.pinned)
  const moved = new Set(movable.flatMap((group) => group.sessions.map((s) => s.id)))
  return reorderSubset(stored, orderGroups(movable, keys), (session) => moved.has(session.id))
}

// neighborSessionId returns the session one step from `sessionId` in the order
// the sidebar draws, wrapping at both ends. "" means there is nowhere to go —
// unknown project, no sessions, or a single one — and the caller leaves focus
// alone. A `sessionId` no longer in the list (its session was closed) lands on
// the end the step comes from, so the press still moves somewhere.
//
// The order is the grouped one, not the stored flat list: a session opened in
// the project root after a worktree one sits between its own group's cards in
// the state and under them on screen, so walking the flat list would jump a
// divider and come back. Pinned cards, and the split's, are walked where they
// are drawn — in the block at the top, not in the worktree they belong to.
export function neighborSessionId(
  state: SessionState,
  projectId: string,
  sessionId: string,
  step: 1 | -1,
  stage: readonly PaneGroup[] = [],
): string {
  const sessions = sidebarGroups(sessionsOf(state, projectId), stage).flatMap(
    (group) => group.sessions,
  )
  if (sessions.length < 2) {
    return ""
  }
  const index = sessions.findIndex((s) => s.id === sessionId)
  if (index === -1) {
    return step === 1 ? sessions[0].id : sessions[sessions.length - 1].id
  }
  return sessions[(index + step + sessions.length) % sessions.length].id
}

// setSessionPinned pins or unpins a session, leaving the list order alone —
// sortPinned is what hoists it, at render time. Unknown project or session ids
// are ignored, returning the input state unchanged.
export function setSessionPinned(
  state: SessionState,
  projectId: string,
  sessionId: string,
  pinned: boolean,
): SessionState {
  const current = state[projectId]
  if (!current || !current.sessions.some((s) => s.id === sessionId)) {
    return state
  }
  return {
    ...state,
    [projectId]: {
      ...current,
      sessions: current.sessions.map((s) => (s.id === sessionId ? { ...s, pinned } : s)),
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

export function removeProject(state: SessionState, projectId: string): SessionState {
  if (!(projectId in state)) {
    return state
  }
  const next = { ...state }
  delete next[projectId]
  return next
}

// The provider kinds whose CLI can reopen a conversation by id, mirrored from
// resumeArgs (Go) — keep in sync.
const RESUMABLE_KINDS: readonly SessionKind[] = [
  "claude",
  "codex",
  "antigravity",
  "omp",
  "opencode",
  "crush",
  "cursor",
  "kiro",
]

// The provider kinds whose CLI can branch a conversation rather than only
// reopen it, mirrored from providers.SupportsFork (Go) — keep in sync. Three of
// the eight, and the rest is not a gap lich can close from this side: the offer
// is withheld where the CLI has no verb for it.
const FORKABLE_KINDS: readonly SessionKind[] = ["claude", "codex", "opencode"]

// forkableSession returns the session a "Fork to worktree" offer can be made
// for: one running a provider that forks, carrying the conversation to branch.
// Null for every other card, which is what keeps the menu item off it — a row
// that could only fail is worse than no row.
//
// Whether that conversation is still on disk is the backend's answer
// (ResumeAvailable), asked by the caller when the offer is taken up rather than
// on every render of every card.
export function forkableSession(session: Session): Session | null {
  if (!FORKABLE_KINDS.includes(session.kind) || !session.providerSessionId) {
    return null
  }
  return session
}

// resumableSession returns the session whose PTY should ask before it spawns,
// because it carries the provider conversation it ran before the last restart.
// Null for everything with nothing to resume: unknown ids, sessions created in
// this run, providers with no resume wired, and shell sessions — whose shell
// cannot reopen a conversation even when a hand-run provider CLI left an id on
// their row.
export function resumableSession(
  state: SessionState,
  projectId: string,
  sessionId: string,
): Session | null {
  const session = state[projectId]?.sessions.find((s) => s.id === sessionId)
  if (!session || !RESUMABLE_KINDS.includes(session.kind) || !session.providerSessionId) {
    return null
  }
  return session
}

export function sessionsOf(state: SessionState, projectId: string): Session[] {
  return state[projectId]?.sessions ?? []
}

// activeTarget resolves what a project screen acts on: the active session's id,
// the path it lives in — a worktree session resolves to its checkout, everything
// else to the project root — which provider it runs, and whether it is confined
// (the footer's attach button hands a confined session a copy instead of a path
// it cannot open). Pure, and the whole of
// useActiveSession's answer: every screen inside a project — the terminal, its
// settings, its pull requests — reads this same triple, and so does the chrome
// beside them. kind is "" when no session is active, which is not a SessionKind:
// a screen with nothing running has no provider, and "shell" would be a claim.
export function activeTarget(
  state: SessionState,
  projectId: string | null,
  projectPath: string,
): { sessionId: string; path: string; kind: SessionKind | ""; sandboxed: boolean } {
  if (!projectId) {
    return { sessionId: "", path: projectPath, kind: "", sandboxed: false }
  }
  const sessionId = activeSessionId(state, projectId)
  const session = sessionsOf(state, projectId).find((s) => s.id === sessionId)
  return {
    sessionId,
    path: session?.path || projectPath,
    kind: session?.kind ?? "",
    sandboxed: session?.sandboxed ?? false,
  }
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

// The project root group's stand-in id. Its path is the empty string, which a
// dnd-kit sortable id cannot be.
export const ROOT_GROUP_KEY = "__root__"

function groupKey(path: string): string {
  return path || ROOT_GROUP_KEY
}

// orderGroups returns the flat session-id order that lays the groups out in the
// given key order, each group keeping its own internal order. Group order is not
// stored anywhere — groupByWorktree reads it off the flat list — so moving a
// group means moving its whole block of ids. A key naming no group contributes
// nothing, which makes the result fail reorderSessions' id-set check rather than
// silently drop that group's sessions.
export function orderGroups(groups: SidebarGroup[], keys: string[]): string[] {
  const byKey = new Map(groups.map((group) => [group.key, group]))
  return keys.flatMap((key) => byKey.get(key)?.sessions.map((session) => session.id) ?? [])
}

// True only for the last session in a worktree checkout. Removing a checkout a
// sibling session still occupies would throw away its work, so only the last
// occupant gets offered the keep/remove prompt.
export function isLastWorktreeSession(sessions: Session[], session: Session): boolean {
  if (!session.path) {
    return false
  }
  return !sessions.some((s) => s.id !== session.id && s.path === session.path)
}

// hasSession reports whether the workspace still holds a session, under any
// project. It answers one question: whether a card that is going away is going
// away because its session ended, which is the only reason its PTY may be
// closed — React unmounts a terminal for reasons of its own.
export function hasSession(state: SessionState, sessionId: string): boolean {
  return projectOfSession(state, sessionId) !== ""
}

// sessionOrigin names the session a card was opened from, as the sidebar should
// spell it now: the label the parent answers to at this moment, so a rename
// shows through without a reload, falling back to the name it had when the
// delegation happened. "" for a session nobody delegated — the usual case — and
// for one whose parent is gone without a name ever having been recorded.
//
// The lookup spans every project because delegation does: a card can hand work
// to a session in another project, and its own project alone would answer "" for
// a parent that plainly exists.
export function sessionOrigin(state: SessionState, session: Session): string {
  if (!session.originSessionId) {
    return ""
  }
  const projectId = projectOfSession(state, session.originSessionId)
  const parent = state[projectId]?.sessions.find((s) => s.id === session.originSessionId)
  return parent?.label ?? session.originLabel ?? ""
}

// delegatesOf returns the sessions this one handed work to, in the project's own
// order. Direct delegates only: `originSessionId` records who asked, and walking
// the chain further would gather grandchildren the user never watched being
// spawned — "its delegates" is the ones it made, not everything downstream of
// them.
//
// Scoped to one project, unlike sessionOrigin: a wall draws sessions of the
// project it belongs to, so a delegate in another project is not something the
// stage could show even if the delegation crossed over.
export function delegatesOf(state: SessionState, projectId: string, sessionId: string): Session[] {
  if (!sessionId) {
    return []
  }
  return sessionsOf(state, projectId).filter((session) => session.originSessionId === sessionId)
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
