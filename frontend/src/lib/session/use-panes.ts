import { useProjects } from "@/providers/projects"
import { activeSessionId, sessionsOf } from "./sessions"
import { fits, nextCandidate, resolveStage, swapCells } from "./panes"
import { stageSize, useStoredStage, writeStage } from "./panes-store"

export interface Panes {
  /** Session ids in layout order, always holding the focused one. */
  cells: string[]
  /** Index of the cell drawing the active session. */
  focus: number
  split: boolean
  /** Move the cursor to a cell. */
  focusCell: (index: number) => void
  /** Move the cursor one cell along, wrapping. */
  focusStep: (step: number) => void
  /** Show one more session — a named one, or the next card not already up.
   * False when there is none left to show, or when one more would leave them
   * all too small to read. */
  add: (sessionId?: string) => boolean
  /** Stop showing the session in this cell. Never closes it. */
  drop: (index: number) => void
  /** Swap two cells, which is what a pane dropped on another one does. */
  swap: (from: number, to: number) => void
}

// The one definition of what the stage does, shared by the terminals that draw
// it and the layout that binds its shortcuts. Every move here is the same two
// writes — the cells and which one holds the cursor — and a second copy of that
// pair somewhere else is the thing most likely to drift.
export function usePanes(projectId: string): Panes {
  const { sessions, activateSession } = useProjects()
  const list = sessionsOf(sessions, projectId)
  const activeId = activeSessionId(sessions, projectId)
  const { cells, focus } = resolveStage(useStoredStage(projectId), list, activeId)

  // Moving the cursor is one write, and it is not to the stage: the focused cell
  // is wherever the active session sits in the list, so activating that session
  // is the whole move.
  const focusCell = (index: number) => {
    const id = cells[index]
    if (!projectId || !id || index === focus) {
      return
    }
    activateSession(projectId, id)
  }

  return {
    cells,
    focus,
    split: cells.length > 1,
    focusCell,
    focusStep(step) {
      if (cells.length < 2) {
        return
      }
      focusCell((focus + step + cells.length) % cells.length)
    },
    add(sessionId) {
      const id = sessionId ?? nextCandidate(list, cells)
      const { width, height } = stageSize()
      if (!projectId || !id || cells.includes(id)) {
        return false
      }
      if (!fits(cells.length + 1, width, height)) {
        return false
      }
      writeStage(projectId, [...cells, id])
      return true
    },
    drop(index) {
      if (!projectId || index < 0 || index >= cells.length) {
        return
      }
      const rest = cells.filter((_, at) => at !== index)
      // Dropping the focused pane hands the cursor to its neighbour rather than
      // leaving the window on a session it no longer draws.
      const next = Math.min(focus, rest.length - 1)
      writeStage(projectId, rest)
      if (index === focus && rest[next]) {
        activateSession(projectId, rest[next])
      }
    },
    swap(from, to) {
      if (!projectId || from === to) {
        return
      }
      // The cursor follows the pane it was in rather than the place it sat,
      // which needs no move of its own: the focused cell is read from where the
      // active session lands in the new order.
      writeStage(projectId, swapCells(cells, from, to))
    },
  }
}
