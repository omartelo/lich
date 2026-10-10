import { Plus, SquareTerminal } from "lucide-react"
import { useParams } from "react-router-dom"
import { EmptyScreen } from "@/components/common/EmptyScreen"
import { Button } from "@/components/ui/button"
import { useProjects } from "@/providers/projects"
import { useProjectSessionKind } from "@/lib/providers-store"
import { sessionsOf } from "@/lib/session/sessions"
import { useT } from "@/lib/i18n/i18n"

// A sessionless project is a legal state: the user is asked for a session rather
// than having a replacement PTY spawned behind their back. The route matches for
// every project, so the emptiness gate lives here — the router cannot express it
// without covering the running terminals underneath.
export function EmptySessions() {
  const t = useT()
  const { sessions, newSession } = useProjects()
  const { projectId = "" } = useParams()
  // What the button will actually spawn is decided in the store, for every
  // implicit entry point at once. This only reads the same answer, so the label
  // cannot promise an agent the machine has not got.
  const noAgent = useProjectSessionKind(projectId) === "shell"

  if (sessionsOf(sessions, projectId).length > 0) {
    return null
  }

  return (
    <EmptyScreen
      icon={SquareTerminal}
      title={t("shell.emptySessions.title")}
      description={
        noAgent ? t("shell.emptySessions.noAgent") : t("shell.emptySessions.description")
      }
    >
      <Button onClick={() => newSession(projectId)}>
        {noAgent ? <SquareTerminal data-icon="inline-start" /> : <Plus data-icon="inline-start" />}
        {noAgent ? t("shell.emptySessions.newTerminal") : t("shell.emptySessions.newSession")}
      </Button>
    </EmptyScreen>
  )
}
