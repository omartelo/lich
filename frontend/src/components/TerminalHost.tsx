import { useMatch } from "react-router-dom"
import { TerminalView } from "@/components/TerminalView"

// Open projects. Fixed to a single "home" project (PTY in the user's home dir)
// until the multi-project rail lands; every terminal here stays mounted so its
// PTY survives navigation.
export const PROJECTS = [{ id: "home", label: "home" }] as const

// TerminalHost keeps one persistent terminal per open project stacked in the
// same area. The router only decides which one is visible — terminals are never
// unmounted by navigation, so background sessions keep running. Inactive layers
// use visibility:hidden (not display:none) so they retain layout size and fit()
// stays correct.
export function TerminalHost() {
  const match = useMatch("/projects/:id")
  const activeId = match?.params.id ?? null

  return (
    <>
      {PROJECTS.map((project) => {
        const visible = project.id === activeId
        return (
          <div
            key={project.id}
            className="absolute inset-0"
            style={{ visibility: visible ? "visible" : "hidden" }}
            aria-hidden={!visible}
          >
            <TerminalView projectId={project.id} visible={visible} />
          </div>
        )
      })}
    </>
  )
}
