import { useMatch } from "react-router-dom"
import { useProjects } from "@/providers/projects"
import { activeTarget } from "./sessions"

// useActiveSession resolves what is currently in focus: the routed project, its
// active session and the working path that session lives in — a worktree
// session resolves to its checkout, everything else to the project root. The
// footer and the review panel follow this triple; projectId is null on
// project-less screens (Home), where sessionId and path are empty.
//
// The match is the project's whole subtree, not the bare route: Settings and
// the pull-request screen are that project's screens too, and the chrome around
// them — the footer, the dock beside them — follows the same session it did on
// the terminal. Matching exactly is what used to empty the dock the moment the
// user opened a pull request, which is where its file tree is needed most.
export function useActiveSession(): {
  projectId: string | null
  sessionId: string
  path: string
} {
  const { projects, sessions } = useProjects()
  const match = useMatch({ path: "/projects/:projectId", end: false })
  const projectId = match?.params.projectId ?? null
  const projectPath = projects.find((p) => p.id === projectId)?.path ?? ""
  return { projectId, ...activeTarget(sessions, projectId, projectPath) }
}
