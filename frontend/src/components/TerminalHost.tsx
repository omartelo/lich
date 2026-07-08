import { useMatch } from "react-router-dom"
import { TerminalView } from "@/components/TerminalView"
import { useProjects } from "@/lib/projects"

// TerminalHost keeps one persistent terminal per open project stacked in the
// same area. The router only decides which one is visible — terminals are never
// unmounted by navigation, so background sessions keep running. Inactive layers
// use visibility:hidden (not display:none) so they retain layout size and fit()
// stays correct.
export function TerminalHost() {
  const { projects } = useProjects()
  const match = useMatch("/projects/:id")
  const activeId = match?.params.id ?? null

  return (
    <>
      {projects.map((project) => {
        const visible = project.id === activeId
        return (
          <div
            key={project.id}
            className="absolute inset-0"
            style={{ visibility: visible ? "visible" : "hidden" }}
            aria-hidden={!visible}
          >
            <TerminalView
              projectId={project.id}
              cwd={project.path}
              visible={visible}
            />
          </div>
        )
      })}
    </>
  )
}
