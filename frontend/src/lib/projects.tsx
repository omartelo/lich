import { createContext, useCallback, useContext, useEffect, useRef, useState } from "react"
import type { ReactNode } from "react"
import { toast } from "sonner"
import { useMatch, useNavigate } from "react-router-dom"
import type { Project } from "./api-types"
import type { StoredProject as StoreProject } from "./api-types"
import { ProjectService, Store } from "./rpc"
import { onAppEvent } from "./app-events"
import {
  activeSessionId,
  addSession,
  closeSession as removeSession,
  projectOfSession,
  removeProject,
  renameSession as relabelSession,
  reorderSessions as rearrangeSessions,
  sessionsOf,
  setActiveSession,
  type SessionKind,
  type SessionState,
} from "./sessions"
import { applyOrder, pinFirst } from "./reorder"
import {
  isIdEvent,
  isStatusEvent,
  isTitleEvent,
  shouldToastAttention,
  STATUS_EVENT,
  TITLE_EVENT,
  toSessionStatus,
  TOUCHED_EVENT,
} from "./session-events"
import { refreshGitStatus } from "./useGitStatus"
import { markSessionSeen } from "./useSessionStatus"
import { isRecordingTarget, matchesCombo } from "./hotkeys"
import { useSettings } from "./settings"

interface ProjectsValue {
  projects: Project[]
  /** Sessions keyed by project id, with the active session per project. */
  sessions: SessionState
  /** The pinned Home tab's project id, or null until resolved at launch. */
  homeId: string | null
  /** Show the OS directory picker, add the chosen project and navigate to it. */
  openProject: () => Promise<void>
  /** Ensure a project rooted at $HOME exists (no picker) and return its id — the
   * update flow's install terminal when no project is in view. */
  ensureHomeProject: () => Promise<string>
  /** Close a project's tab (kept in the store so it can be reopened later). */
  closeProject: (id: string) => void
  /** Open a new session in a project and focus it, returning its id. Kind
   * defaults to Claude Code. */
  newSession: (projectId: string, kind?: SessionKind) => string
  /** Open a Claude Code session rooted at a git worktree, labeled after it. */
  newWorktreeSession: (projectId: string, wt: { name: string; path: string }) => void
  /** Permanently delete a session; deleting the last one recreates an empty one. */
  closeSession: (projectId: string, sessionId: string) => void
  /** Focus an existing session within a project. */
  activateSession: (projectId: string, sessionId: string) => void
  /** Rename a session's display label. */
  renameSession: (projectId: string, sessionId: string, label: string) => void
  /** Rearrange the project tabs to the given id order (drag-and-drop). */
  reorderProjects: (ids: string[]) => void
  /** Rearrange a project's session cards to the given id order (drag-and-drop). */
  reorderSessions: (projectId: string, ids: string[]) => void
}

const ProjectsContext = createContext<ProjectsValue | null>(null)

const newSessionId = (): string => crypto.randomUUID()

// The first session of any project is always "Session 1"; the counter then
// points at 2 for the next one.
const FIRST_LABEL = "Session 1"
const FIRST_NEXT_SEQ = 2

const toProject = (p: StoreProject): Project => ({
  id: p.id,
  name: p.name,
  path: p.path,
})

// How long the "needs you" toast stays before auto-dismissing.
const ATTENTION_TOAST_MS = 10_000

// Shown when a toasted session has no label to name it by.
const UNLABELED_SESSION = "A session"

// buildSessionState rebuilds the in-memory session map from the persisted
// projects returned by the store.
function buildSessionState(loaded: StoreProject[]): SessionState {
  const state: SessionState = {}
  for (const p of loaded) {
    const sessions = (p.sessions ?? []).map((s) => ({
      id: s.id,
      label: s.label,
      kind: (s.kind === "shell" ? "shell" : "claude") as SessionKind,
      ...(s.path ? { path: s.path } : {}),
      ...(s.claudeSessionId ? { claudeSessionId: s.claudeSessionId } : {}),
    }))
    state[p.id] = {
      sessions,
      activeId: p.activeSessionId || sessions[0]?.id || "",
      nextSeq: p.nextSeq,
    }
  }
  return state
}

// ProjectsProvider is the write-through layer over the SQLite store: it mirrors
// every mutation to the store and hydrates from it on launch so open projects
// and their sessions survive restarts. In-project mutations read the latest
// rendered session state through sessionsRef, which is safe because none of them
// awaits a state-changing call before reading it.
export function ProjectsProvider({ children }: { children: ReactNode }) {
  const [projects, setProjects] = useState<Project[]>([])
  const [sessions, setSessions] = useState<SessionState>({})
  const sessionsRef = useRef(sessions)
  sessionsRef.current = sessions
  const projectsRef = useRef(projects)
  projectsRef.current = projects
  // The always-present Home tab's project id, resolved at launch — a pinned,
  // non-closable shell at the system home dir.
  const [homeId, setHomeId] = useState<string | null>(null)
  const homeIdRef = useRef<string | null>(null)
  homeIdRef.current = homeId
  const navigate = useNavigate()
  // "/*" so a project stays the active one while its Settings screen is open
  // (keeps the new-session hotkey, attention toasts and seen-tracking working).
  const activeProjectId = useMatch("/projects/:projectId/*")?.params.projectId
  // Latest focused project id for the attention toast, read inside a once-only
  // event subscription without re-subscribing on every navigation.
  const activeProjectIdRef = useRef(activeProjectId)
  activeProjectIdRef.current = activeProjectId
  const { hotkeys } = useSettings()

  const applyLoaded = useCallback((loaded: StoreProject[]) => {
    setProjects(loaded.map(toProject))
    setSessions(buildSessionState(loaded))
  }, [])

  // Restore the workspace once on launch, and seed the always-present Home tab:
  // a plain shell rooted at the system home dir. It is a normal (persisted)
  // project, so its sessions and terminals reuse all the usual machinery; the
  // tab strip just pins it first, non-closable, with a Home icon.
  useEffect(() => {
    void (async () => {
      let loaded = (await Store.LoadState()) ?? []
      const home = await ProjectService.Home().catch(() => null)
      if (home) {
        setHomeId(home.id)
        const existing = loaded.find((p) => p.id === home.id)
        if (!existing) {
          await Store.AddProject(home.id, home.name, home.path)
          await Store.AddSession(home.id, newSessionId(), FIRST_LABEL, "shell", "", FIRST_NEXT_SEQ)
          loaded = (await Store.LoadState()) ?? []
        } else if ((existing.sessions ?? []).length === 0) {
          await Store.AddSession(home.id, newSessionId(), FIRST_LABEL, "shell", "", FIRST_NEXT_SEQ)
          loaded = (await Store.LoadState()) ?? []
        }
      }
      applyLoaded(loaded)
    })()
  }, [applyLoaded])

  // The backend auto-applies the Claude ai-title as a session's label (only
  // while the user has not renamed it) and emits this event with the change.
  // Mirror it into local state so the card updates live; the store already
  // persisted it, so this never writes back.
  useEffect(() => {
    const off = onAppEvent(TITLE_EVENT, (data) => {
      if (!isTitleEvent(data)) {
        return
      }
      const { id, label } = data
      const projectId = projectOfSession(sessionsRef.current, id)
      if (!projectId) {
        return
      }
      const next = relabelSession(sessionsRef.current, projectId, id, label)
      if (next !== sessionsRef.current) {
        setSessions(next)
      }
    })
    return () => off()
  }, [])

  const openProject = useCallback(async () => {
    const picked = await ProjectService.Open()
    if (!picked) {
      return
    }
    await Store.AddProject(picked.id, picked.name, picked.path)

    // Reload from the store to pick up any sessions a reopened project kept, and
    // seed a first session when it is brand new (or was left empty).
    let loaded = (await Store.LoadState()) ?? []
    const mine = loaded.find((p) => p.id === picked.id)
    if (!mine || (mine.sessions ?? []).length === 0) {
      await Store.AddSession(picked.id, newSessionId(), FIRST_LABEL, "claude", "", FIRST_NEXT_SEQ)
      loaded = (await Store.LoadState()) ?? []
    }
    applyLoaded(loaded)
    navigate(`/projects/${picked.id}`)
  }, [applyLoaded, navigate])

  // Add a $HOME-rooted project without the picker, idempotent by its stable id
  // (the same directory always maps to the same project), and return its id. The
  // update flow opens an install terminal here when nothing is in view; the
  // caller adds the shell session so no default session is seeded.
  const ensureHomeProject = useCallback(async (): Promise<string> => {
    if (homeIdRef.current) {
      return homeIdRef.current
    }
    const home = await ProjectService.Home()
    setHomeId(home.id)
    if (!projectsRef.current.some((p) => p.id === home.id)) {
      setProjects((prev) => (prev.some((p) => p.id === home.id) ? prev : [...prev, home]))
      await Store.AddProject(home.id, home.name, home.path)
    }
    return home.id
  }, [])

  const closeProject = useCallback(
    (id: string) => {
      if (id === homeIdRef.current) {
        return // Home is permanent.
      }
      const index = projects.findIndex((project) => project.id === id)
      setProjects((prev) => prev.filter((project) => project.id !== id))
      setSessions((prev) => removeProject(prev, id))
      void Store.CloseProject(id)
      // Closing a background tab leaves focus untouched; closing the active one
      // falls back to the previous tab (then the next, then Home when none left).
      if (activeProjectId !== id) {
        return
      }
      const neighbor = projects[index - 1] ?? projects[index + 1]
      navigate(neighbor ? `/projects/${neighbor.id}` : "/")
    },
    [projects, activeProjectId, navigate],
  )

  const newSession = useCallback((projectId: string, kind: SessionKind = "claude") => {
    const sessionId = newSessionId()
    const next = addSession(sessionsRef.current, projectId, sessionId, kind)
    const project = next[projectId]
    const created = project.sessions[project.sessions.length - 1]
    setSessions(next)
    void Store.AddSession(projectId, sessionId, created.label, kind, "", project.nextSeq)
    return sessionId
  }, [])

  const newWorktreeSession = useCallback(
    (projectId: string, wt: { name: string; path: string }) => {
      const sessionId = newSessionId()
      const next = addSession(sessionsRef.current, projectId, sessionId, "claude", wt.path, wt.name)
      const project = next[projectId]
      const created = project.sessions[project.sessions.length - 1]
      setSessions(next)
      void Store.AddSession(projectId, sessionId, created.label, "claude", wt.path, project.nextSeq)
    },
    [],
  )

  // The new-session shortcut opens a session in the active project (mirrors the
  // "+" button). It fires even with terminal focus, so it stays reachable while
  // working in a terminal — the capture-phase listener sees the chord before
  // the terminal does and stops propagation so the PTY never receives it.
  // Bails while a hotkey is being recorded so rebinding does not trigger it.
  useEffect(() => {
    if (!activeProjectId) {
      return
    }
    const onKey = (event: KeyboardEvent) => {
      if (isRecordingTarget(event)) return
      if (matchesCombo(event, hotkeys.newSession)) {
        event.preventDefault()
        event.stopPropagation()
        newSession(activeProjectId)
      }
    }
    window.addEventListener("keydown", onKey, true)
    return () => window.removeEventListener("keydown", onKey, true)
  }, [activeProjectId, newSession, hotkeys])

  const closeSession = useCallback((projectId: string, sessionId: string) => {
    const removed = removeSession(sessionsRef.current, projectId, sessionId)
    if (removed === sessionsRef.current) {
      return
    }
    // A project always keeps at least one session; recreate when emptied. Home
    // stays a shell so it never turns into a Claude session.
    if (sessionsOf(removed, projectId).length === 0) {
      const recreatedId = newSessionId()
      const kind: SessionKind = projectId === homeIdRef.current ? "shell" : "claude"
      const next = addSession(removed, projectId, recreatedId, kind)
      const project = next[projectId]
      const created = project.sessions[project.sessions.length - 1]
      setSessions(next)
      void Store.DeleteSession(projectId, sessionId, "").then(() =>
        Store.AddSession(projectId, recreatedId, created.label, created.kind, "", project.nextSeq),
      )
      return
    }
    setSessions(removed)
    void Store.DeleteSession(projectId, sessionId, activeSessionId(removed, projectId))
  }, [])

  const activateSession = useCallback((projectId: string, sessionId: string) => {
    const next = setActiveSession(sessionsRef.current, projectId, sessionId)
    if (next === sessionsRef.current) {
      return
    }
    setSessions(next)
    void Store.SetActiveSession(projectId, sessionId)
  }, [])

  // A session that needs the user (permission prompt or idle input) raises a
  // global toast that routes to its card — reachable even when the session lives
  // in a background project whose card is not mounted. Skipped for the session
  // already in focus, where the terminal itself shows the prompt.
  //
  // Driven off the raw event rather than the status store on purpose: the store
  // collapses a repeat state into no notification, which would swallow the toast
  // for a second waiting report. One toast per report is the contract here.
  useEffect(() => {
    const off = onAppEvent(STATUS_EVENT, (data) => {
      if (!isStatusEvent(data) || toSessionStatus(data.state) !== "waiting") {
        return
      }
      const { id } = data
      if (!shouldToastAttention(sessionsRef.current, id, activeProjectIdRef.current)) {
        return
      }
      const projectId = projectOfSession(sessionsRef.current, id)
      const label =
        sessionsRef.current[projectId]?.sessions.find((s) => s.id === id)?.label ??
        UNLABELED_SESSION
      toast(`${label} needs your input`, {
        duration: ATTENTION_TOAST_MS,
        action: {
          label: "Open",
          onClick: () => {
            navigate(`/projects/${projectId}`)
            activateSession(projectId, id)
          },
        },
      })
    })
    return () => off()
  }, [navigate, activateSession])

  // Opening a project puts its cards on screen, so its sessions' statuses count
  // as seen — and the cleanup marks them again on the way out, with the project
  // being left. Without that second pass, a turn that finished while you sat in
  // the project would badge the tab you just walked away from. A turn still
  // running keeps its tab badged either way: only "done" reads this.
  useEffect(() => {
    if (!activeProjectId) {
      return
    }
    const markSeen = () => {
      for (const session of sessionsOf(sessionsRef.current, activeProjectId)) {
        markSessionSeen(session.id)
      }
    }
    markSeen()
    return markSeen
  }, [activeProjectId])

  // A session that likely changed files on disk nudges an immediate git-status
  // refresh for the path its card watches (its worktree, else the project's),
  // ahead of the steady 3s poll. The poll still runs, so a user without the
  // plugin keeps the same feedback — this only cuts the lag when the hook fires.
  useEffect(() => {
    const off = onAppEvent(TOUCHED_EVENT, (data) => {
      if (!isIdEvent(data)) {
        return
      }
      const { id } = data
      const projectId = projectOfSession(sessionsRef.current, id)
      if (!projectId) {
        return
      }
      const session = sessionsRef.current[projectId]?.sessions.find(
        (s) => s.id === id,
      )
      const project = projectsRef.current.find((p) => p.id === projectId)
      const path = session?.path || project?.path
      if (path) {
        refreshGitStatus(path)
      }
    })
    return () => off()
  }, [])

  const reorderProjects = useCallback((ids: string[]) => {
    // Home is pinned first and rendered outside the drag list, so the drop only
    // names the other projects; splice it back in (when present) so applyOrder
    // still accounts for every project.
    const hid = homeIdRef.current
    const inProjects = hid !== null && projectsRef.current.some((p) => p.id === hid)
    const full = pinFirst(ids, inProjects ? hid : null)
    const next = applyOrder(projectsRef.current, full)
    if (!next) {
      return
    }
    setProjects(next)
    void Store.ReorderProjects(full)
  }, [])

  const reorderSessions = useCallback((projectId: string, ids: string[]) => {
    const next = rearrangeSessions(sessionsRef.current, projectId, ids)
    if (next === sessionsRef.current) {
      return
    }
    setSessions(next)
    void Store.ReorderSessions(projectId, ids)
  }, [])

  const renameSession = useCallback(
    (projectId: string, sessionId: string, label: string) => {
      const next = relabelSession(sessionsRef.current, projectId, sessionId, label)
      if (next === sessionsRef.current) {
        return
      }
      setSessions(next)
      void Store.RenameSession(sessionId, label)
    },
    [],
  )

  return (
    <ProjectsContext.Provider
      value={{
        projects,
        sessions,
        homeId,
        openProject,
        ensureHomeProject,
        closeProject,
        newSession,
        newWorktreeSession,
        closeSession,
        activateSession,
        renameSession,
        reorderProjects,
        reorderSessions,
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
