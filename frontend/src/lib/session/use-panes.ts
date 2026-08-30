import { useProjects } from "@/providers/projects"
import { activeSessionId, sessionsOf } from "./sessions"
import {
  addToGroup,
  defaultName,
  dissolveGroup,
  groupOf,
  movingFrom,
  nextCandidate,
  type PaneGroup,
  removeFromGroups,
  resolveGroups,
  swapCells,
  updateGroup,
} from "./panes"
import { fits } from "./pane-grid"
import { stageSize, useStoredGroups, writeGroups } from "./panes-store"

export interface Panes {
  /** Every wall in this project, reconciled, in the order the sidebar draws. */
  groups: PaneGroup[]
  /** The wall the active session belongs to, or null when it is on none. */
  current: PaneGroup | null
  /** Session ids the window is drawing, and where the cursor is among them. */
  cells: string[]
  focus: number
  split: boolean
  /** Move the cursor to a cell of the wall on screen. */
  focusCell: (index: number) => void
  /** Move the cursor one cell along, wrapping. */
  focusStep: (step: number) => void
  /** Show one more session — a named one, or the next card on no wall. Adds to
   * the wall the active session is on, or starts a new one around it. False when
   * there is nothing left to show, when one more would leave them all too small
   * to read, or when that session is on another wall and `move` was not asked
   * for. That last refusal is the guard: taking a session off somebody else's
   * arrangement is a decision for the user, so a caller has to have asked them
   * first — one that forgets gets a no-op rather than a silent move. */
  add: (sessionId?: string, opts?: { move?: boolean }) => boolean
  /** Take a session off its wall. Never closes it. */
  remove: (sessionId: string) => void
  /** Take the session in this cell of the wall on screen off it. */
  drop: (index: number) => void
  /** Swap two cells of the wall on screen. */
  swap: (from: number, to: number) => void
  /** Set a wall's whole pane order — what dragging its cards in the sidebar
   * does, from the other end of the same arrangement. */
  reorderCells: (groupId: string, ids: string[]) => void
  rename: (groupId: string, name: string) => void
  dissolve: (groupId: string) => void
  /** Persist a wall's dragged column or row shares. */
  setTracks: (groupId: string, change: { cols?: number[]; rows?: number[] }) => void
  /** Reorder the walls themselves, which is what dragging their blocks does. */
  reorder: (groups: PaneGroup[]) => void
}

// The one definition of what the walls do, shared by the terminals that draw one
// and the sidebar that lists them all. Every move is the same single write —
// the whole list of groups — and a second copy of that anywhere else is the
// thing most likely to drift.
export function usePanes(projectId: string): Panes {
  const { sessions, activateSession } = useProjects()
  const list = sessionsOf(sessions, projectId)
  const activeId = activeSessionId(sessions, projectId)
  const groups = resolveGroups(useStoredGroups(projectId), list)
  const current = activeId ? groupOf(groups, activeId) : null
  const cells = current ? current.cells : activeId ? [activeId] : []
  const focus = Math.max(cells.indexOf(activeId), 0)

  const commit = (next: PaneGroup[]) => writeGroups(projectId, next)

  const remove = (sessionId: string) => {
    const group = groupOf(groups, sessionId)
    if (!projectId || !group) {
      return
    }
    commit(removeFromGroups(groups, sessionId))
    // Taking away the pane that held the cursor hands it to what is still up,
    // rather than leaving the window on a session it no longer draws. Only when
    // that pane *was* the cursor: removing a member of a parked wall changes
    // nothing about what is on screen.
    if (sessionId !== activeId) {
      return
    }
    const rest = group.cells.filter((id) => id !== sessionId)
    if (rest.length > 0) {
      const at = group.cells.indexOf(sessionId)
      activateSession(projectId, rest[Math.min(at, rest.length - 1)])
    }
  }

  return {
    groups,
    current,
    cells,
    focus,
    split: cells.length > 1,
    focusCell(index) {
      const id = cells[index]
      if (projectId && id && index !== focus) {
        activateSession(projectId, id)
      }
    },
    focusStep(step) {
      if (cells.length < 2) {
        return
      }
      const id = cells[(focus + step + cells.length) % cells.length]
      if (projectId && id) {
        activateSession(projectId, id)
      }
    },
    add(sessionId, opts) {
      const id = sessionId ?? nextCandidate(list, groups)
      const { width, height } = stageSize()
      if (!projectId || !id || id === activeId || !activeId) {
        return false
      }
      if (movingFrom(groups, current, id) && !opts?.move) {
        return false
      }
      if (!fits(cells.length + 1, width, height)) {
        return false
      }
      if (current) {
        commit(addToGroup(groups, current.id, id))
        return true
      }
      // On no wall: this starts one, named after the session it grew from —
      // which in the flow this exists for is the orchestrator that spawned the
      // rest. It never replaces another group; the project keeps as many as the
      // user makes.
      const born: PaneGroup = {
        id: newGroupId(),
        name: defaultName(list, activeId),
        cells: [activeId, id],
        cols: [],
        rows: [],
      }
      commit([...removeFromGroups(groups, id), born])
      return true
    },
    remove,
    drop(index) {
      remove(cells[index] ?? "")
    },
    swap(from, to) {
      if (projectId && current && from !== to) {
        commit(updateGroup(groups, current.id, { cells: swapCells(current.cells, from, to) }))
      }
    },
    reorderCells(groupId, ids) {
      commit(updateGroup(groups, groupId, { cells: ids }))
    },
    rename(groupId, name) {
      const trimmed = name.trim()
      if (projectId && trimmed) {
        commit(updateGroup(groups, groupId, { name: trimmed }))
      }
    },
    dissolve(groupId) {
      commit(dissolveGroup(groups, groupId))
    },
    setTracks(groupId, change) {
      commit(updateGroup(groups, groupId, change))
    },
    reorder(next) {
      commit(next)
    },
  }
}

// Group ids only have to be unique inside one project's pref, and they are minted
// where the impurity belongs — panes.ts stays a pure module the suite can drive.
function newGroupId(): string {
  return `g${Date.now().toString(36)}${Math.random().toString(36).slice(2, 6)}`
}
