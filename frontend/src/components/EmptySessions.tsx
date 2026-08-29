import { Plus, SquareTerminal } from "lucide-react"
import { useParams } from "react-router-dom"
import { EmptyScreen } from "@/components/common/EmptyScreen"
import { Button } from "@/components/ui/button"
import { useProjects } from "@/providers/projects"
import { useNoProviderInstalled } from "@/lib/providers-store"
import { sessionsOf } from "@/lib/session/sessions"

// A sessionless project is a legal state: the user is asked for a session rather
// than having a replacement PTY spawned behind their back. The route matches for
// every project, so the emptiness gate lives here — the router cannot express it
// without covering the running terminals underneath.
export function EmptySessions() {
  const { sessions, newSession } = useProjects()
  const { projectId = "" } = useParams()
  // What the button will actually spawn is decided in the store, for every
  // implicit entry point at once. This only reads the same answer, so the label
  // cannot promise an agent the machine has not got.
  const noAgent = useNoProviderInstalled()

  if (sessionsOf(sessions, projectId).length > 0) {
    return null
  }

  return (
    <EmptyScreen
      icon={SquareTerminal}
      title="No session open"
      description={
        noAgent
          ? "No coding agent is installed, so this opens a terminal — install one from Settings › Providers."
          : "Open a session to start working in this project."
      }
    >
      <Button onClick={() => newSession(projectId)}>
        {noAgent ? <SquareTerminal data-icon="inline-start" /> : <Plus data-icon="inline-start" />}
        {noAgent ? "New terminal" : "New session"}
      </Button>
    </EmptyScreen>
  )
}
