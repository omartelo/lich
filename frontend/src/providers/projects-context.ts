import { createContext, useContext } from "react"
import type { ClosedSession, Project, RecentProject } from "@/lib/api-types"
import type { Session, SessionKind, SessionState } from "@/lib/session/sessions"

export interface ProjectsValue {
  projects: Project[]
  /** Sessions keyed by project id, with the active session per project. */
  sessions: SessionState
  /** The pinned Home tab's project id, or null until resolved at launch. */
  homeId: string | null
  /** Show the OS directory picker, add the chosen project and navigate to it. */
  openProject: () => Promise<void>
  /** Reopen a closed project without the picker. A project whose directory is
   * gone asks for the new one instead, keeping its id and its sessions. Answers
   * whether the project is open afterwards — a cancelled relocate is a no. */
  openRecent: (recent: RecentProject) => Promise<boolean>
  /** Ensure a project rooted at $HOME exists (no picker) and return its id — the
   * update flow's install terminal when no project is in view. */
  ensureHomeProject: () => Promise<string>
  /** Close a project's tab (kept in the store so it can be reopened later). */
  closeProject: (id: string) => void
  /** Open a new session in a project and focus it, returning its id. Kind
   * defaults to the project's provider choice; path to its own directory. */
  newSession: (projectId: string, kind?: SessionKind, path?: string) => string
  /** Open a project-default session rooted at a git worktree, labeled after it,
   * returning its id. sandbox is the dialog's confinement answer ("on"/"off"),
   * "" to follow the provider's rung. from is the session being forked into this
   * checkout, which lends the new card its provider and its lineage. */
  newWorktreeSession: (
    projectId: string,
    wt: { name: string; path: string },
    sandbox?: string,
    from?: Session | null,
  ) => string
  /** Resume a worktree: reopen its parked session when one exists, else open a
   * fresh session on it. Answers with the session either way, for a caller with
   * something to hand it. */
  reopenWorktreeSession: (projectId: string, wt: { name: string; path: string }) => Promise<string>
  /** Resume a session picked out of the history, reopening its project first
   * when that project's tab is gone. */
  resumeClosedSession: (closed: ClosedSession) => Promise<void>
  /** Close a session, parking its row so the history can resume it. Offers an
   * Undo toast for a stray click. */
  closeSession: (projectId: string, sessionId: string) => void
  /** Delete a session for good, with no undo offered — the close that takes the
   * checkout with it, where a parked row would outlive its directory. */
  discardSession: (projectId: string, sessionId: string) => void
  /** Close a worktree session but park its row so it can be resumed later. */
  keepSession: (projectId: string, sessionId: string) => void
  /** Focus an existing session within a project. */
  activateSession: (projectId: string, sessionId: string) => void
  /** Rename a session's display label. */
  renameSession: (projectId: string, sessionId: string, label: string) => void
  // Record the command a terminal session opens into, "" to clear it. Refused
  // by the store on anything but a shell session.
  setEntrypoint: (projectId: string, sessionId: string, entrypoint: string) => void
  // Park a prompt to be typed at a session at unix second `at`, or clear the
  // one it holds with at 0. One per session: this replaces what was there.
  scheduleSession: (sessionId: string, at: number, prompt: string) => void
  /** Pin a session to the head of its project's list, or unpin it. */
  pinSession: (projectId: string, sessionId: string, pinned: boolean) => void
  /** Rearrange the project tabs to the given id order. */
  reorderProjects: (ids: string[]) => void
  /** Rearrange a project's session cards to the given id order. */
  reorderSessions: (projectId: string, ids: string[]) => void
}

export const ProjectsContext = createContext<ProjectsValue | null>(null)

export function useProjects(): ProjectsValue {
  const context = useContext(ProjectsContext)
  if (!context) {
    throw new Error("useProjects must be used within a ProjectsProvider")
  }
  return context
}
