import { Plus, SquareTerminal } from "lucide-react"
import { useParams } from "react-router-dom"
import { Button } from "@/components/ui/button"
import { useProjects } from "@/lib/projects"
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
    <div className="absolute inset-0 z-10 flex flex-col items-center justify-center gap-4 bg-background">
      <SquareTerminal className="size-12 text-muted-foreground" />
      <div className="text-center">
        <h1 className="text-lg font-semibold text-foreground">No session open</h1>
        <p className="text-sm text-muted-foreground">
          Open a session to start working in this project.
        </p>
      </div>
      <Button onClick={() => newSession(projectId)}>
        <Plus data-icon="inline-start" />
        New session
      </Button>
    </div>
  )
}
