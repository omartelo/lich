import { Plus, SquareTerminal } from "lucide-react"
import { useParams } from "react-router-dom"
import { EmptyScreen } from "@/components/common/EmptyScreen"
import { Button } from "@/components/ui/button"
import { useProjects } from "@/providers/projects"
import { sessionsOf } from "@/lib/sessions"

// A sessionless project is a legal state: the user is asked for a session rather
// than having a replacement PTY spawned behind their back. The route matches for
// every project, so the emptiness gate lives here — the router cannot express it
// without covering the running terminals underneath.
export function EmptySessions() {
  const { sessions, newSession } = useProjects()
  const { projectId = "" } = useParams()

  if (sessionsOf(sessions, projectId).length > 0) {
    return null
  }

  return (
    <EmptyScreen
      icon={SquareTerminal}
      title="No session open"
      description="Open a session to start working in this project."
    >
      <Button onClick={() => newSession(projectId)}>
        <Plus data-icon="inline-start" />
        New session
      </Button>
    </EmptyScreen>
  )
}
