import { useMatch } from "react-router-dom"
import { useProjects } from "@/providers/projects"
import { activeTarget } from "./sessions"
import { useSessionCwd } from "./use-session-cwd"

// useActiveSession resolves what is currently in focus: the routed project, its
// active session and the working path that session lives in — the session's
// live cwd, falling back to its checkout (a worktree) and then the project
// root. The footer and the review panel follow this triple; projectId is null
// on project-less screens (Home), where sessionId and path are empty.
//
// The cwd overlay is what keeps the dock on the same tree the session card and
// the footer show: a session spawned in one checkout and `cd`-ed into another
// reviews where it is working, not where it started — the static path can be a
// clean worktree while every count on screen comes from somewhere else.
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
  /** The session's static checkout — what the sidebar keys a worktree's pull
   * request card on, so a `cd` cannot open a card no group can show. */
  checkout: string
} {
  const { projects, sessions } = useProjects()
  const match = useMatch({ path: "/projects/:projectId", end: false })
  const projectId = match?.params.projectId ?? null
  const projectPath = projects.find((p) => p.id === projectId)?.path ?? ""
  const target = activeTarget(sessions, projectId, projectPath)
  const cwd = useSessionCwd(target.sessionId)
  return {
    projectId,
    sessionId: target.sessionId,
    path: cwd || target.path,
    checkout: target.path,
  }
}
