import { useState } from "react"
import { type PaneGroup, stageAction } from "./panes"
import type { Session } from "./sessions"
import type { Panes } from "./use-panes"

interface StageMove {
  session: Session
  from: PaneGroup
}

export function useStageToggle(panes: Panes, sessions: Session[]) {
  const [moving, setMoving] = useState<StageMove | null>(null)

  const toggleStage = (sessionId: string) => {
    const action = stageAction(panes.groups, panes.current, sessionId)
    const session = sessions.find((held) => held.id === sessionId)
    if (action.kind === "confirm" && session) {
      setMoving({ session, from: action.from })
    } else if (action.kind === "remove") {
      panes.remove(sessionId)
    } else {
      panes.add(sessionId)
    }
  }

  const confirmMove = () => {
    if (moving) {
      panes.add(moving.session.id, { move: true })
    }
    setMoving(null)
  }

  return { moving, toggleStage, cancelMove: () => setMoving(null), confirmMove }
}
