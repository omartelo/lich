import { createContext, useCallback, useContext, useState } from "react"
import type { ReactNode } from "react"
import { useNavigate } from "react-router-dom"
import {
  Project,
  Service as ProjectService,
} from "../../bindings/github.com/skipodotdev/skipo/internals/project"

interface ProjectsValue {
  projects: Project[]
  /** Show the OS directory picker, add the chosen project and navigate to it. */
  openProject: () => Promise<void>
  /** Remove a project (unmounts its terminal, which closes the PTY). */
  closeProject: (id: string) => void
}

const ProjectsContext = createContext<ProjectsValue | null>(null)

// ponytail: open projects are kept in memory only. Persist to a config file
// (and restore on launch) when projects should survive restarts.
export function ProjectsProvider({ children }: { children: ReactNode }) {
  const [projects, setProjects] = useState<Project[]>([])
  const navigate = useNavigate()

  const openProject = useCallback(async () => {
    const picked = await ProjectService.Open()
    if (!picked) {
      return
    }
    setProjects((prev) =>
      prev.some((project) => project.id === picked.id) ? prev : [...prev, picked],
    )
    navigate(`/projects/${picked.id}`)
  }, [navigate])

  const closeProject = useCallback(
    (id: string) => {
      setProjects((prev) => prev.filter((project) => project.id !== id))
      navigate("/")
    },
    [navigate],
  )

  return (
    <ProjectsContext.Provider value={{ projects, openProject, closeProject }}>
      {children}
    </ProjectsContext.Provider>
  )
}

export function useProjects(): ProjectsValue {
  const ctx = useContext(ProjectsContext)
  if (!ctx) {
    throw new Error("useProjects must be used within a ProjectsProvider")
  }
  return ctx
}
