import { useProjects } from "@/providers/projects"
import { activeSessionId, sessionsOf } from "./sessions"
import { fits, nextCandidate, resolveStage, swapCells } from "./panes"
import { stageSize, useStoredStage, writeStage } from "./panes-store"

export interface Panes {
  /** Every session on the stage, whether or not the wall is what is on screen —
   * what the sidebar marks. */
  members: string[]
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
  /** Take a session off the stage by id — the card's own way off, which has to
   * work while the wall is parked and there are no cells to index. */
  remove: (sessionId: string) => void
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
  const { cells, focus, members } = resolveStage(useStoredStage(projectId), list, activeId)

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

  const remove = (sessionId: string) => {
    if (!projectId || !members.includes(sessionId)) {
      return
    }
    const next = members.filter((id) => id !== sessionId)
    writeStage(projectId, next)
    // Taking away the pane that held the cursor hands it to what is still up,
    // rather than leaving the window on a session it no longer draws. Only when
    // that pane *was* the cursor: removing a parked member changes nothing about
    // what is on screen.
    if (sessionId === activeId && next.length > 0) {
      const at = members.indexOf(sessionId)
      activateSession(projectId, next[Math.min(at, next.length - 1)])
    }
  }

  return {
    members,
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
      remove(cells[index] ?? "")
    },
    remove,
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
