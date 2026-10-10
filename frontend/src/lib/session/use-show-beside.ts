import { toast } from "sonner"
import { useProjects } from "@/providers/projects"
import { t } from "@/lib/i18n/i18n"
import { besideAction } from "./panes"
import { stageSize } from "./panes-store"
import { workspaceSessions } from "./sessions"
import { requestStageMove } from "./stage-move-store"
import { usePanes } from "./use-panes"

// useShowBeside is what the palette's Alt+Enter and a pane's + share: put a
// session from any open project beside the one on screen. Null while the routed
// project has no active session, since there is nothing to show it beside —
// the callers hide their affordance rather than offer one that does nothing.
//
// Unlike the add shortcut, a refusal is said: the user named this session, so a
// no-op would read as the click being lost.
export function useShowBeside(projectId: string): ((sessionId: string) => void) | null {
  const { sessions } = useProjects()
  const panes = usePanes(projectId)
  const activeId = panes.cells[panes.focus] ?? ""
  if (!activeId) {
    return null
  }
  return (sessionId) => {
    const action = besideAction(panes.groups, panes.current, activeId, sessionId, stageSize())
    switch (action.kind) {
      case "showing":
        return
      case "full":
        toast(t("terminal.host.noRoom"))
        return
      case "confirm": {
        const session = workspaceSessions(sessions).find((held) => held.id === sessionId)
        if (session) {
          requestStageMove({ session, from: action.from })
        }
        return
      }
      case "add":
        panes.add(sessionId)
    }
  }
}
