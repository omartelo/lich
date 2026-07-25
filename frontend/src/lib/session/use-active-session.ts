import { useMatch } from "react-router-dom"
import { useProjects } from "@/providers/projects"
import { activeTarget } from "./sessions"

// useActiveSession resolves what is currently in focus: the routed project, its
// active session and the working path that session lives in — a worktree
// session resolves to its checkout, everything else to the project root. The
// footer and the review panel follow this triple; projectId is null on
// project-less screens (Home, Settings), where sessionId and path are empty.
export function useActiveSession(): {
  projectId: string | null
  sessionId: string
  path: string
} {
  const { projects, sessions } = useProjects()
  const match = useMatch("/projects/:projectId")
  const projectId = match?.params.projectId ?? null
  const projectPath = projects.find((p) => p.id === projectId)?.path ?? ""
  return { projectId, ...activeTarget(sessions, projectId, projectPath) }
}
