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
// It has a name, it survives its members being closed down to the last one, and
// it ends only when they dissolve it or when nothing is left in it. Which is why
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
// A group of one survives: it is still something the user named and can add to.
// A group of none does not — its last member is gone, so there is nothing left
// for the name to be about.
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
    if (cells.length > 0) {
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
export function grouped(groups: readonly PaneGroup[]): Set<string> {
  return new Set(groups.flatMap((group) => group.cells))
}

// defaultName is the label of the session the group was started from, which in
// the flow this exists for is the orchestrator that spawned the rest. A name the
// user can read beats "Split 2", and one they can retype beats a name lich
// insists on.
export function defaultName(sessions: readonly Session[], sessionId: string): string {
  return sessions.find((session) => session.id === sessionId)?.label || "Split"
}

/** Put a session on a wall, taking it off whatever wall had it. */
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
    .filter((group) => group.cells.length > 0)
}

/** Take a session off whatever wall it is on. The group stays, even at one
 * member; only its last member leaving ends it. */
export function removeFromGroups(groups: readonly PaneGroup[], sessionId: string): PaneGroup[] {
  return groups
    .map((group) => ({ ...group, cells: group.cells.filter((id) => id !== sessionId) }))
    .filter((group) => group.cells.length > 0)
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

/** Reorder the groups themselves, which is what dragging their blocks does. */
export function reorderGroups(groups: readonly PaneGroup[], ids: readonly string[]): PaneGroup[] {
  const byId = new Map(groups.map((group) => [group.id, group]))
  const moved = ids.flatMap((id) => byId.get(id) ?? [])
  // Anything the drag did not name keeps its place, at the end rather than being
  // dropped: an id set that raced a dissolve must not cost the user a group.
  const named = new Set(ids)
  return [...moved, ...groups.filter((group) => !named.has(group.id))]
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
export function nextCandidate(sessions: readonly Session[], groups: readonly PaneGroup[]): string {
  const taken = grouped(groups)
  return sessions.find((session) => !taken.has(session.id))?.id ?? ""
}
