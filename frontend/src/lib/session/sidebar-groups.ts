// How a project's sessions are drawn in the sidebar: the blocks they are
// gathered into, and the flat id orders a drag inside or between those blocks
// commits back to the stored list.
//
// Kept apart from sessions.ts, which owns the state itself: nothing here mutates
// a session, and every function is pure over the list the reducers hand it.

import type { PaneGroup } from "./panes"
import type { PendingStatus } from "./session-status-store"
import { sessionsOf, type Session, type SessionState } from "./sessions"

// The pinned block's stand-in id, for the same reason ROOT_GROUP_KEY exists: it
// is not a path. The block gathers sessions from every checkout, so it can
// collide with no worktree — those paths are absolute.
export const PINNED_GROUP_KEY = "__pinned__"

// A folder block's key. Prefixed for the same reason the two stand-ins above
// exist: a folder gathers cards from every checkout and is named by the user, so
// a folder called "/home/me/project" must not key the block a real checkout of
// that path already keys. The name rides in the key rather than being looked up
// elsewhere, which is what lets the fold pref (group-prefs) and the drag list
// key a folder without a second lookup.
export const FOLDER_KEY_PREFIX = "__folder__:"

export const folderKey = (name: string): string => `${FOLDER_KEY_PREFIX}${name}`

// One block of the sidebar: a split group, the pinned sessions, a folder, or one
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
  // The folder this block draws, "" for every other kind. Like a wall it
  // gathers cards from any checkout — which is the whole point of filing by
  // subject — so it has no path either, and the name is what the header wears
  // and what a rename rewrites on every session in it.
  folder: string
  // The checkout root ("" for the project's own directory), empty for the
  // gathered blocks.
  path: string
  sessions: Session[]
}

// sidebarGroups splits a project's sessions into the blocks the sidebar draws:
// the pinned ones first — that is what a pin promises — then the walls, the
// folders and the worktrees interleaved, each block landing where its first card
// sits in the stored list. Each block keeps that stored (drag) order inside,
// except a wall's, which keeps the order of the panes it draws.
//
// One order for every kind of block, read off the one list, is what lets a drag
// move a folder past a worktree: block order is nowhere else, so moving a block
// is moving its cards inside the flat list and nothing has a second opinion.
//
// Neither a split nor a pin nor a folder rewrites that list; each only lifts a
// card into a block, which is what lets a session dropped from any of them land
// back among its old neighbours instead of being stranded. The store hands the
// same order back, so a reload draws what the live change did.
//
// Precedence where a session could be in two blocks is wall, then pin, then
// folder, then its checkout: a session both filed and on a wall is drawn in the
// wall, and one both filed and pinned is drawn at the top — the pin promises the
// head of the list rather than one particular header, and the card keeps its own
// mark either way. A wall or a folder with nothing left in it draws nothing: a
// folder is the set of sessions carrying its name, so the last card leaving is
// what ends it.
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
  // What a session is drawn as, once and for all, so the bucketing below and the
  // walk that orders the blocks can never disagree about where a card went.
  const gathered = (session: Session): "wall" | "pin" | "folder" | "checkout" => {
    if (wallOf.has(session.id)) {
      return "wall"
    }
    if (session.pinned) {
      return "pin"
    }
    return session.folder ? "folder" : "checkout"
  }

  const blocks: SidebarGroup[] = []
  const pinned = sessions.filter((session) => gathered(session) === "pin")
  if (pinned.length > 0) {
    blocks.push({
      key: PINNED_GROUP_KEY,
      pinned: true,
      stage: null,
      folder: "",
      path: "",
      sessions: pinned,
    })
  }

  const byFolder = new Map<string, Session[]>()
  for (const session of sessions) {
    if (gathered(session) === "folder") {
      const members = byFolder.get(session.folder ?? "")
      if (members) {
        members.push(session)
      } else {
        byFolder.set(session.folder ?? "", [session])
      }
    }
  }
  const loose = sessions.filter((session) => gathered(session) === "checkout")
  const byPath = new Map(
    groupByWorktree(loose).map((group) => [groupKey(group.path), group] as const),
  )
  const drawn = new Set<string>()
  for (const session of sessions) {
    const where = gathered(session)
    if (where === "pin") {
      continue
    }
    const wall = wallOf.get(session.id)
    const folder = session.folder ?? ""
    const key =
      where === "wall" && wall
        ? wall.id
        : where === "folder"
          ? folderKey(folder)
          : groupKey(session.path ?? "")
    if (drawn.has(key)) {
      continue
    }
    drawn.add(key)
    if (where === "wall" && wall) {
      blocks.push({
        key,
        pinned: false,
        stage: wall,
        folder: "",
        path: "",
        sessions: wall.cells.flatMap((id) => byId.get(id) ?? []),
      })
      continue
    }
    if (where === "folder") {
      blocks.push({
        key,
        pinned: false,
        stage: null,
        folder,
        path: "",
        sessions: byFolder.get(folder) ?? [],
      })
      continue
    }
    const group = byPath.get(key)
    if (group) {
      blocks.push({
        key,
        pinned: false,
        stage: null,
        folder: "",
        path: group.path,
        sessions: group.sessions,
      })
    }
  }
  return blocks
}

// collapsedMark is what a folded block says about the cards it is hiding: a
// session waiting on the user, else a finished turn nobody has read, else
// nothing. Waiting outranks done for the reason it does on the card — the amber
// state is the one asking for something — and neither says how many, because a
// block that is folded has already been told to take one line.
//
// It reads the notification queue rather than each card's ring: that queue is
// already "everything worth surfacing", so a status that stops counting as news
// stops lighting the block with no second rule here.
export function collapsedMark(
  pending: readonly PendingStatus[],
  ids: readonly string[],
): "wait" | "done" | null {
  let done = false
  for (const entry of pending) {
    if (!ids.includes(entry.id)) {
      continue
    }
    if (entry.status === "waiting") {
      return "wait"
    }
    done = done || entry.status === "done"
  }
  return done ? "done" : null
}

// runCardIn names the Run card of the checkout stored as `path` ("" for the
// project's own directory), or undefined when it has none, which is what puts
// the launch menu's item on "Run" rather than "Go to Run card".
//
// It reads the project's whole list rather than the block that checkout draws: a
// Run card that has been pinned, or dragged onto a wall, is drawn in one of the
// gathered blocks and still holds its checkout's slot, exactly as the backend
// sees it (internal/spawn.runCardIn matches the row, not the block).
export function runCardIn(sessions: Session[], path: string): Session | undefined {
  return sessions.find((session) => session.run && (session.path ?? "") === path)
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
