import { stageAction } from "./panes"
import type { Session } from "./sessions"
import { requestStageMove } from "./stage-move-store"
import type { Panes } from "./use-panes"

export function useStageToggle(panes: Panes, sessions: Session[]) {
  const toggleStage = (sessionId: string) => {
    const action = stageAction(panes.groups, panes.current, sessionId)
    const session = sessions.find((held) => held.id === sessionId)
    if (action.kind === "confirm" && session) {
      requestStageMove({ session, from: action.from })
    } else if (action.kind === "remove") {
      panes.remove(sessionId)
    } else {
      panes.add(sessionId)
    }
  }

  return { toggleStage }
}
