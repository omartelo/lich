import { useMatch } from "react-router-dom"
import { TerminalView } from "@/components/TerminalView"
import { useProjects } from "@/lib/projects"
import { activeSessionId, sessionsOf } from "@/lib/sessions"

// TerminalHost keeps one persistent terminal per session, across every open
// project, stacked in the same area. The router picks the active project and the
// per-project active session decides which layer is visible — terminals are
// never unmounted by navigation, so background sessions keep running. Inactive
// layers use visibility:hidden (not display:none) so they retain layout size and
// fit() stays correct.
export function TerminalHost() {
  const { projects, sessions } = useProjects()
  const match = useMatch("/projects/:projectId")
  const activeProjectId = match?.params.projectId ?? null

  return (
    <>
      {projects.flatMap((project) => {
        const projectActiveId = activeSessionId(sessions, project.id)
        return sessionsOf(sessions, project.id).map((session) => {
          const visible =
            project.id === activeProjectId && session.id === projectActiveId
          return (
            <div
              key={session.id}
              className="absolute inset-0"
              style={{ visibility: visible ? "visible" : "hidden" }}
              aria-hidden={!visible}
            >
              <TerminalView
                sessionId={session.id}
                cwd={project.path}
                visible={visible}
              />
            </div>
          )
        })
      })}
    </>
  )
}
