import { useState } from "react"
import { movingFrom, type PaneGroup } from "./panes"
import type { Session } from "./sessions"
import type { Panes } from "./use-panes"

interface StageMove {
  session: Session
  from: PaneGroup
}

export function useStageToggle(panes: Panes, sessions: Session[]) {
  const [moving, setMoving] = useState<StageMove | null>(null)

  const toggleStage = (sessionId: string) => {
    const from = movingFrom(panes.groups, panes.current, sessionId)
    const session = sessions.find((held) => held.id === sessionId)
    if (from && session) {
      setMoving({ session, from })
    } else if (panes.current?.cells.includes(sessionId)) {
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
