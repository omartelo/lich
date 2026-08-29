import { useProjects } from "@/providers/projects"
import { activeSessionId, sessionsOf } from "./sessions"
import { besideCandidate, other, resolveBeside, type Side } from "./panes"
import { closeBeside, openBeside, useStoredBeside } from "./panes-store"

export interface Panes {
  /** The session in the second pane, "" when the stage is not split. */
  beside: string
  /** Which half that session draws on; the focused one takes the other. */
  besideSide: Side
  split: boolean
  /** Swap which pane holds the keyboard. */
  focusOther: () => void
  /** Drop the focused pane and keep the one beside it. */
  promoteOther: () => void
  /** Split with the next card, or close the split when there is one. False when
   * the project has nothing to put beside — the shortcut then declines rather
   * than splitting a pane against itself. */
  toggle: () => boolean
}

// The one definition of what the panes do, shared by the stage that draws them
// and the layout that binds their shortcuts. Both halves would otherwise carry
// their own copy of the swap, which is the pair of writes most likely to drift:
// focusing the other pane is *two* stores agreeing — the project's active
// session and the beside id trade places — and a second implementation that
// wrote only one of them would leave a card showing in both panes.
export function usePanes(projectId: string): Panes {
  const { sessions, activateSession } = useProjects()
  const list = sessionsOf(sessions, projectId)
  const activeId = activeSessionId(sessions, projectId)
  const beside = resolveBeside(useStoredBeside(projectId), list, activeId)

  return {
    beside: beside.id,
    besideSide: beside.side,
    split: beside.id !== "",
    // The two writes are one move: the session that was focused becomes the
    // beside one *on the side it was already drawing on*, so the terminals hold
    // still and only the cursor crosses over.
    focusOther() {
      if (!projectId || !beside.id) {
        return
      }
      openBeside(projectId, { id: activeId, side: other(beside.side) })
      activateSession(projectId, beside.id)
    },
    promoteOther() {
      if (!projectId || !beside.id) {
        return
      }
      activateSession(projectId, beside.id)
      closeBeside(projectId)
    },
    toggle() {
      if (!projectId) {
        return false
      }
      if (beside.id) {
        closeBeside(projectId)
        return true
      }
      const next = besideCandidate(list, activeId)
      if (!next) {
        return false
      }
      openBeside(projectId, { id: next, side: "right" })
      return true
    },
  }
}
