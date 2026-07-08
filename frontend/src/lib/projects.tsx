import { createContext, useCallback, useContext, useState } from "react"
import type { ReactNode } from "react"
import { useNavigate } from "react-router-dom"
import {
  Project,
  Service as ProjectService,
} from "../../bindings/github.com/skipodotdev/skipo/internals/project"
import {
  addSession,
  closeSession as removeSession,
  removeProject,
  sessionsOf,
  setActiveSession,
  type SessionState,
} from "./sessions"

interface ProjectsValue {
  projects: Project[]
  /** Sessions keyed by project id, with the active session per project. */
  sessions: SessionState
  /** Show the OS directory picker, add the chosen project and navigate to it. */
  openProject: () => Promise<void>
  /** Remove a project (unmounts its terminals, which closes their PTYs). */
  closeProject: (id: string) => void
  /** Open a new terminal session in a project and focus it. */
  newSession: (projectId: string) => void
  /** Close a session; closing the last one recreates an empty one. */
  closeSession: (projectId: string, sessionId: string) => void
  /** Focus an existing session within a project. */
  activateSession: (projectId: string, sessionId: string) => void
}

const ProjectsContext = createContext<ProjectsValue | null>(null)

const newSessionId = (): string => crypto.randomUUID()

// ponytail: open projects and their sessions are kept in memory only. Persist to
// a config file (and restore on launch) when they should survive restarts.
export function ProjectsProvider({ children }: { children: ReactNode }) {
  const [projects, setProjects] = useState<Project[]>([])
  const [sessions, setSessions] = useState<SessionState>({})
  const navigate = useNavigate()

  const openProject = useCallback(async () => {
    const picked = await ProjectService.Open()
    if (!picked) {
      return
    }
    setProjects((prev) =>
      prev.some((project) => project.id === picked.id) ? prev : [...prev, picked],
    )
    setSessions((prev) =>
      prev[picked.id] ? prev : addSession(prev, picked.id, newSessionId()),
    )
    navigate(`/projects/${picked.id}`)
  }, [navigate])

  const closeProject = useCallback(
    (id: string) => {
      setProjects((prev) => prev.filter((project) => project.id !== id))
      setSessions((prev) => removeProject(prev, id))
      navigate("/")
    },
    [navigate],
  )

  const newSession = useCallback((projectId: string) => {
    setSessions((prev) => addSession(prev, projectId, newSessionId()))
  }, [])

  const closeSession = useCallback((projectId: string, sessionId: string) => {
    setSessions((prev) => {
      const next = removeSession(prev, projectId, sessionId)
      // A project always keeps at least one session; recreate when emptied.
      return sessionsOf(next, projectId).length === 0
        ? addSession(next, projectId, newSessionId())
        : next
    })
  }, [])

  const activateSession = useCallback((projectId: string, sessionId: string) => {
    setSessions((prev) => setActiveSession(prev, projectId, sessionId))
  }, [])

  return (
    <ProjectsContext.Provider
      value={{
        projects,
        sessions,
        openProject,
        closeProject,
        newSession,
        closeSession,
        activateSession,
      }}
    >
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
