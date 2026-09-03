// Split groups: named walls of sessions the user assembles, several per project.
//
// The model is one sentence: a pane is a viewport onto a card that already
// exists, never a session it owns. That is what makes this unlike a terminal
// emulator's split, where a pane *is* a shell because there is nowhere else to
// keep one — lich already has as many sessions as the user wants and the sidebar
// is the multiplexer, so the only thing a wall buys is seeing several at once.
// A pane's × stops showing a session and closes nothing.
//
// A group is a thing the user made, not the current arrangement of the window.
// It has a name, it survives any one member leaving, and it ends when they
// dissolve it or when fewer than two members are left — a wall of one is a
// session on its own, and the survivor goes back to its checkout. Which is why
// a project holds a list of them: an orchestrator and the three worktrees it
// spawned are one group, the next investigation and its two are another, and
// neither is "the" split.
//
// Two rules keep "which wall is on screen" from ever being ambiguous:
//
//   - a session belongs to at most one group, so adding it to another moves it;
//   - the group on screen is the one holding the active session, and none is
//     when the active session is in no group. A session activated from anywhere
//     — a card, the palette, a link in another session's output, an MCP call —
//     therefore shows on its own and parks whatever wall was up, whole. Only the
//     add affordance ever puts a session in a group.
//
// Nothing here remembers a focused cell. The cursor is the active session and
// its cell is wherever that id sits in the group, so focus is read rather than
// stored and a stale one cannot outlive the cell it named.
import { applyOrder } from "@/lib/reorder"
import { fits } from "./pane-grid"
import type { Session } from "./sessions"

export interface PaneGroup {
  /** Stable across renames and reorders: the sidebar keys its block, its fold
   * and its drag off this, and a name the user can retype is none of those. */
  id: string
  name: string
  /** Session ids, in the order they are laid out. */
  cells: string[]
  /** Column and row shares, belonging to the group rather than to the window:
   * arranging one wall 60/40 must not carry into the next one. */
  cols: number[]
  rows: number[]
}

export const groupsKey = (projectId: string): string => `lich.panes.${projectId}`

function numbers(value: unknown): number[] {
  return Array.isArray(value) ? value.filter((n): n is number => typeof n === "number") : []
}

// parseGroups reads the stored value, and answers "no groups" for anything it
// does not recognise. A pref must never be able to break a launch (prefs.ts), so
// a truncated write, a value from another build, or hand-edited JSON costs the
// user their arrangement and nothing else.
export function parseGroups(raw: string | null): PaneGroup[] {
  if (!raw) {
    return []
  }
  try {
    const parsed: unknown = JSON.parse(raw)
    if (!Array.isArray(parsed)) {
      return []
    }
    return parsed.flatMap((value) => {
      const group = value as Partial<PaneGroup>
      if (typeof group?.id !== "string" || !Array.isArray(group.cells)) {
        return []
      }
      return [
        {
          id: group.id,
          name: typeof group.name === "string" ? group.name : "",
          cells: group.cells.filter((id): id is string => typeof id === "string"),
          cols: numbers(group.cols),
          rows: numbers(group.rows),
        },
      ]
    })
  } catch {
    return []
  }
}

export function formatGroups(groups: readonly PaneGroup[]): string {
  return JSON.stringify(groups)
}

// resolveGroups reconciles what is stored against the project as it is now, and
// is the one guard the whole feature needs.
//
// Membership is derived on every read rather than maintained on every mutation:
// a session leaves the workspace through a close, a park, a worktree removal and
// an undone create, and a list kept in step with all four is four chances to
// leave a pane pointing at a dead id. Answering "is it still in the list" at
// read time is right for every one of those paths, including the ones added
// after this. The same pass enforces the one-group rule, so a stored value that
// somehow holds a session twice cannot put it on two walls.
//
// A group needs two live members to still be a group. A wall of one draws
// nothing a solo session does not, so its survivor is better off back among its
// own checkout's cards than alone under a header it cannot fill — and the group
// the pane left is gone whichever way it emptied: a close, a park, a worktree
// removal, a move onto another wall.
export function resolveGroups(
  stored: readonly PaneGroup[],
  sessions: readonly Session[],
): PaneGroup[] {
  const live = new Set(sessions.map((session) => session.id))
  const claimed = new Set<string>()
  const groups: PaneGroup[] = []
  for (const group of stored) {
    const cells: string[] = []
    for (const id of group.cells) {
      if (live.has(id) && !claimed.has(id)) {
        claimed.add(id)
        cells.push(id)
      }
    }
    if (cells.length > 1) {
      groups.push({ ...group, cells })
    }
  }
  return groups
}

/** The group a session belongs to, or null. */
export function groupOf(groups: readonly PaneGroup[], sessionId: string): PaneGroup | null {
  return groups.find((group) => group.cells.includes(sessionId)) ?? null
}

// movingFrom answers the one question adding a session has to ask first: is it
// already on somebody else's wall? Taking it off that one is a change to an
// arrangement the click was not aimed at, so it is the user's decision and not
// lich's — the group it would leave is returned so they can be told which, and
// whether losing this member ends it.
//
// One definition, because both halves need the same answer: the sidebar asks it
// to raise the prompt, and `add` asks it to refuse until that prompt has been
// answered. A caller that forgets to ask gets a no-op rather than a silent move.
export function movingFrom(
  groups: readonly PaneGroup[],
  current: PaneGroup | null,
  sessionId: string,
): PaneGroup | null {
  const held = groupOf(groups, sessionId)
  return held && held !== current ? held : null
}

/** Every session on any wall — what the add shortcut must not offer again. */
function grouped(groups: readonly PaneGroup[]): Set<string> {
  return new Set(groups.flatMap((group) => group.cells))
}

// defaultName is the label of the session the group was started from, which in
// the flow this exists for is the orchestrator that spawned the rest. A name the
// user can read beats "Split 2", and one they can retype beats a name lich
// insists on.
export function defaultName(sessions: readonly Session[], sessionId: string): string {
  return sessions.find((session) => session.id === sessionId)?.label || "Split"
}

/** Put a session on a wall, taking it off whatever wall had it — and ending
 * that one if this leaves it below two. */
export function addToGroup(
  groups: readonly PaneGroup[],
  groupId: string,
  sessionId: string,
): PaneGroup[] {
  return groups
    .map((group) =>
      group.id === groupId
        ? { ...group, cells: [...group.cells.filter((id) => id !== sessionId), sessionId] }
        : { ...group, cells: group.cells.filter((id) => id !== sessionId) },
    )
    .filter((group) => group.cells.length > 1)
}

/** Take a session off whatever wall it is on. A wall left with one member ends
 * with it: there is no split left to be a group about. */
export function removeFromGroups(groups: readonly PaneGroup[], sessionId: string): PaneGroup[] {
  return groups
    .map((group) => ({ ...group, cells: group.cells.filter((id) => id !== sessionId) }))
    .filter((group) => group.cells.length > 1)
}

export function updateGroup(
  groups: readonly PaneGroup[],
  groupId: string,
  change: Partial<Omit<PaneGroup, "id">>,
): PaneGroup[] {
  return groups.map((group) => (group.id === groupId ? { ...group, ...change } : group))
}

export function dissolveGroup(groups: readonly PaneGroup[], groupId: string): PaneGroup[] {
  return groups.filter((group) => group.id !== groupId)
}

/** Swap two cells — what a pane dropped on another one does. */
export function swapCells(cells: readonly string[], from: number, to: number): string[] {
  if (from === to || from < 0 || to < 0 || from >= cells.length || to >= cells.length) {
    return [...cells]
  }
  const next = [...cells]
  next[from] = cells[to]
  next[to] = cells[from]
  return next
}

// nextCandidate picks what the add shortcut shows: the first card in the
// project's own order that is on no wall at all. The sidebar's order is the one
// the user arranged, so what arrives is the one they can predict, and skipping
// what is already grouped keeps the shortcut from quietly moving one wall's
// member onto another.
//
// The active session is skipped with them, and for the same reason it is skipped
// by the caller that refuses to add a session to itself: on no wall it is the
// first card the search would find, and the shortcut that promises to start a
// wall *around* it would answer with the session it is already showing.
export function nextCandidate(
  sessions: readonly Session[],
  groups: readonly PaneGroup[],
  activeId: string,
): string {
  const taken = grouped(groups)
  return sessions.find((session) => session.id !== activeId && !taken.has(session.id))?.id ?? ""
}

/** What adding one more pane comes to: nothing, a session joining the wall on
 * screen, or a new wall around the active session. */
export type AddPlan =
  | { kind: "none" }
  | { kind: "join"; groupId: string; sessionId: string }
  | { kind: "start"; around: string; sessionId: string }

export interface AddRequest {
  sessions: readonly Session[]
  groups: readonly PaneGroup[]
  /** The wall the active session is on, or null when it is on none. */
  current: PaneGroup | null
  activeId: string
  /** The stage as measured, for the "would one more still be readable" guard. */
  stage: { width: number; height: number }
  /** The session to show, or absent for "the next card on no wall". */
  sessionId?: string
  /** Whether the user has agreed to take the session off another wall. */
  move?: boolean
}

// planAdd is the whole refusal matrix of the add affordance, decided before
// anything is written: nothing left to show, no active session to show it
// beside, no room for one more pane, or a session on somebody else's wall with
// no answer from the user yet. That last refusal is the guard — taking a session
// off an arrangement the click was not aimed at is the user's decision, so a
// caller that forgets to ask gets a no-op rather than a silent move.
export function planAdd(request: AddRequest): AddPlan {
  const { sessions, groups, current, activeId, stage, sessionId, move } = request
  const id = sessionId ?? nextCandidate(sessions, groups, activeId)
  if (!id || !activeId || id === activeId) {
    return { kind: "none" }
  }
  if (movingFrom(groups, current, id) && !move) {
    return { kind: "none" }
  }
  // On no wall the stage draws the active session alone, so one more makes two.
  if (!fits((current ? current.cells.length : 1) + 1, stage.width, stage.height)) {
    return { kind: "none" }
  }
  return current
    ? { kind: "join", groupId: current.id, sessionId: id }
    : { kind: "start", around: activeId, sessionId: id }
}

// focusAfterRemove answers where the cursor goes when the pane holding it is
// taken away: the cell that slid into its place, or the last one when what left
// was the last. "" is "leave the window where it is" — the wall has nothing left
// to hold a cursor, or it never held this session and so lost no pane.
export function focusAfterRemove(cells: readonly string[], sessionId: string): string {
  const at = cells.indexOf(sessionId)
  const rest = cells.filter((id) => id !== sessionId)
  if (at < 0 || rest.length === 0) {
    return ""
  }
  return rest[Math.min(at, rest.length - 1)]
}

/** What the sidebar's one stage entry does for a card, matching what its label
 * offers. `confirm` carries the wall the session would be taken off. */
export type StageAction =
  | { kind: "add" }
  | { kind: "remove" }
  | { kind: "confirm"; from: PaneGroup }

// stageAction reads the card the same way the card's own label does: a session
// drawing in a pane right now is taken off its wall, and any other is put on the
// one on screen. Membership of a *parked* wall is not the question — that card
// is not showing, so the entry reads "Show beside" and this moves it here, after
// the user has answered for the wall it leaves.
export function stageAction(
  groups: readonly PaneGroup[],
  current: PaneGroup | null,
  sessionId: string,
): StageAction {
  if (current?.cells.includes(sessionId)) {
    return { kind: "remove" }
  }
  const from = movingFrom(groups, current, sessionId)
  return from ? { kind: "confirm", from } : { kind: "add" }
}

// reorderCells sets one wall's whole pane order — what dragging its cards in the
// sidebar does. An order that no longer names that wall's exact members is
// dropped whole, the way reorderSessions drops a drag a close raced: a session
// added to the wall while the cards were being dragged is not in the dropped
// order, and writing it would take that session straight back off the wall.
export function reorderCells(
  groups: readonly PaneGroup[],
  groupId: string,
  ids: readonly string[],
): PaneGroup[] {
  const group = groups.find((candidate) => candidate.id === groupId)
  if (!group) {
    return [...groups]
  }
  // applyOrder wants items with an id; a cell is the id itself.
  const ordered = applyOrder(
    group.cells.map((id) => ({ id })),
    [...ids],
  )
  return ordered
    ? updateGroup(groups, groupId, { cells: ordered.map((cell) => cell.id) })
    : [...groups]
}
