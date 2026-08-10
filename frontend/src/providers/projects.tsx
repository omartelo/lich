import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from "react"
import type { ReactNode } from "react"
import { toast } from "sonner"
import { Bell, Folder, MessageSquareDashed } from "lucide-react"
import { useMatch, useNavigate } from "react-router-dom"
import type { Project, RecentProject } from "@/lib/api-types"
import type { StoredProject as StoreProject } from "@/lib/api-types"
import { ProjectService, Store, System } from "@/lib/rpc"
import { onAppEvent } from "@/lib/app-events"
import {
  activeSessionId,
  addSession,
  closeSession as removeSession,
  isSessionKind,
  neighborSessionId,
  projectOfSession,
  removeProject,
  renameSession as relabelSession,
  reorderSessions as rearrangeSessions,
  restoreSession,
  sessionsOf,
  setActiveSession,
  setSessionPinned,
  type Session,
  type SessionKind,
  type SessionState,
} from "@/lib/session/sessions"
import { applyOrder, pinFirst } from "@/lib/reorder"
import { displayPath } from "@/lib/paths"
import { defaultProviderKind } from "@/lib/providers-store"
import {
  RELAY_STALLED_EVENT,
  STATUS_EVENT,
  TITLE_EVENT,
  TOUCHED_EVENT,
  decideStatusNotice,
  isIdEvent,
  isRelayStalledEvent,
  isStatusEvent,
  isTitleEvent,
  shouldToastAttention,
  toSessionStatus,
  type SessionStatus,
} from "@/lib/session/session-events"
import { NotificationsOptIn } from "@/components/NotificationsOptIn"
import { refreshGitStatus } from "@/lib/git/use-git-status"
import { markSessionSeen } from "@/lib/session/use-session-status"
import { isRecordingTarget, matchesCombo } from "@/lib/hotkeys"
import { useSettings } from "./settings"

interface ProjectsValue {
  projects: Project[]
  /** Sessions keyed by project id, with the active session per project. */
  sessions: SessionState
  /** The pinned Home tab's project id, or null until resolved at launch. */
  homeId: string | null
  /** Show the OS directory picker, add the chosen project and navigate to it. */
  openProject: () => Promise<void>
  /** Reopen a closed project without the picker. A project whose directory is
   * gone is dropped from the store instead, with a toast saying so. */
  openRecent: (recent: RecentProject) => Promise<void>
  /** Ensure a project rooted at $HOME exists (no picker) and return its id — the
   * update flow's install terminal when no project is in view. */
  ensureHomeProject: () => Promise<string>
  /** Close a project's tab (kept in the store so it can be reopened later). */
  closeProject: (id: string) => void
  /** Open a new session in a project and focus it, returning its id. Kind
   * defaults to Claude Code; path defaults to the project's own directory. */
  newSession: (projectId: string, kind?: SessionKind, path?: string) => string
  /** Open a Claude Code session rooted at a git worktree, labeled after it,
   * returning its id. */
  newWorktreeSession: (projectId: string, wt: { name: string; path: string }) => string
  /** Resume a worktree: reopen its parked session (continuing its Claude
   * conversation) when one exists, else open a fresh session on it. */
  reopenWorktreeSession: (projectId: string, wt: { name: string; path: string }) => Promise<void>
  /** Permanently delete a session, offering an Undo toast for the stray click;
   * deleting the last one leaves the project with none. */
  closeSession: (projectId: string, sessionId: string) => void
  /** Delete a session with no undo offered — the worktree flows, where the
   * checkout the session lives in is going away with it. */
  discardSession: (projectId: string, sessionId: string) => void
  /** Close a worktree session but park its row so it can be resumed later. */
  keepSession: (projectId: string, sessionId: string) => void
  /** Focus an existing session within a project. */
  activateSession: (projectId: string, sessionId: string) => void
  /** Rename a session's display label. */
  renameSession: (projectId: string, sessionId: string, label: string) => void
  /** Pin a session to the head of its project's list, or unpin it. A pinned
   * session refuses to close until it is unpinned. */
  pinSession: (projectId: string, sessionId: string, pinned: boolean) => void
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

const ATTENTION_TOAST_MS = 10_000

// How long the undo stays on offer after a close. Long enough to notice the card
// is gone and reach the button, short enough that it is not still there when the
// next one is closed on purpose.
const UNDO_TOAST_MS = 10_000

const UNLABELED_SESSION = "A session"

function buildSessionState(loaded: StoreProject[]): SessionState {
  const state: SessionState = {}
  for (const p of loaded) {
    const sessions = (p.sessions ?? []).map((s) => ({
      id: s.id,
      label: s.label,
      kind: isSessionKind(s.kind) ? s.kind : "claude",
      ...(s.path ? { path: s.path } : {}),
      ...(s.providerSessionId ? { providerSessionId: s.providerSessionId } : {}),
      ...(s.pinned ? { pinned: true } : {}),
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
  const { hotkeys, desktopNotifications, setDesktopNotifications, finishedTurnNotifications } =
    useSettings()
  // Read inside the same once-only subscription, for the same reason as the
  // active project: a preference change must not tear down the status listener.
  const desktopNotificationsRef = useRef(desktopNotifications)
  desktopNotificationsRef.current = desktopNotifications
  const finishedTurnNotificationsRef = useRef(finishedTurnNotifications)
  finishedTurnNotificationsRef.current = finishedTurnNotifications
  // The status each session last reported, which decideStatusNotice reads to
  // drop a finished turn repeated with nothing run in between. A closed session
  // leaves its entry behind; one string per session id is not worth a sweep.
  const lastStatusRef = useRef(new Map<string, SessionStatus | null>())
  // Whether the opt-in dialog is up. Raised by the first report that would have
  // notified, never at launch — the question lands with the event that motivates
  // it. Dismissing without an answer leaves the preference unset, so the next
  // one asks again.
  const [askNotifications, setAskNotifications] = useState(false)

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
        // Seeded only on the very first launch: an existing Home left without
        // sessions was emptied on purpose and keeps its empty screen.
        if (!loaded.some((p) => p.id === home.id)) {
          await Store.AddProject(home.id, home.name, home.path)
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

  // A reopened project keeps the sessions it was closed with, hence the reload.
  // Nothing is seeded: a brand-new project lands on the empty screen.
  const adopt = useCallback(
    async (project: Project) => {
      await Store.AddProject(project.id, project.name, project.path)
      applyLoaded((await Store.LoadState()) ?? [])
      navigate(`/projects/${project.id}`)
    },
    [applyLoaded, navigate],
  )

  const openProject = useCallback(async () => {
    const picked = await ProjectService.Open()
    if (!picked) {
      return
    }
    await adopt(picked)
  }, [adopt])

  // Reopening skips the picker, so nothing guarantees the directory is still
  // there — a project whose folder was moved or deleted can never be opened
  // again, so its row goes with it rather than sitting in the list forever.
  const openRecent = useCallback(
    async (recent: RecentProject) => {
      if (await ProjectService.Exists(recent.path)) {
        await adopt(recent)
        return
      }
      await Store.DeleteProject(recent.id)
      toast.error(`Project not found: ${displayPath(recent.path)}`)
    },
    [adopt],
  )

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

  const newSession = useCallback(
    (projectId: string, kind: SessionKind = defaultProviderKind(), path = "") => {
      const sessionId = newSessionId()
      const next = addSession(sessionsRef.current, projectId, sessionId, kind, path)
      const project = next[projectId]
      const created = project.sessions[project.sessions.length - 1]
      setSessions(next)
      void Store.AddSession(projectId, sessionId, created.label, kind, path, project.nextSeq)
      return sessionId
    },
    [],
  )

  const newWorktreeSession = useCallback(
    (projectId: string, wt: { name: string; path: string }) => {
      const sessionId = newSessionId()
      const kind = defaultProviderKind()
      const next = addSession(sessionsRef.current, projectId, sessionId, kind, wt.path, wt.name)
      const project = next[projectId]
      const created = project.sessions[project.sessions.length - 1]
      setSessions(next)
      void Store.AddSession(projectId, sessionId, created.label, kind, wt.path, project.nextSeq)
      return sessionId
    },
    [],
  )

  const reopenWorktreeSession = useCallback(
    async (projectId: string, wt: { name: string; path: string }) => {
      // Bring back the parked session for this worktree if there is one, so its
      // Claude conversation can be resumed; a fresh id means the terminal treats
      // it as unspawned and asks before continuing. No parked row → start fresh.
      const restored = await Store.ReopenWorktreeSession(projectId, wt.path, newSessionId())
      if (!restored) {
        newWorktreeSession(projectId, wt)
        return
      }
      const session: Session = {
        id: restored.id,
        label: restored.label,
        kind: isSessionKind(restored.kind) ? restored.kind : "claude",
        path: restored.path,
        ...(restored.providerSessionId ? { providerSessionId: restored.providerSessionId } : {}),
      }
      setSessions(restoreSession(sessionsRef.current, projectId, session))
    },
    [newWorktreeSession],
  )

  // dropSession removes a session's card and persists that removal via `persist`
  // — DeleteSession (gone for good) or CloseSession (parked for a later resume)
  // — returning that write so a caller can chain on it. Closing the last one
  // leaves the project sessionless: the EmptySessions screen then offers a new
  // one, rather than a replacement PTY spawning unasked.
  const dropSession = useCallback(
    (
      projectId: string,
      sessionId: string,
      persist: (activeID: string) => Promise<unknown>,
    ): Promise<unknown> => {
      const removed = removeSession(sessionsRef.current, projectId, sessionId)
      if (removed === sessionsRef.current) {
        return Promise.resolve()
      }
      setSessions(removed)
      return persist(activeSessionId(removed, projectId))
    },
    [],
  )

  // Undo re-creates the row a close deleted, from what the page still holds,
  // rather than parking it for the toast's lifetime: parking would need a
  // reopen-by-id the store has no other reason to grow, and every app exit
  // inside the toast window would strand a hidden parked row that a later
  // worktree resume could resurrect. Re-creating owns no window — the close is
  // final the moment it is made, and the undo is a plain new write. What the
  // row does not carry back: label_auto (a restored card is auto-titleable
  // again, even if its name was hand-picked) and the cost ledgers of any
  // transcript it will not run through again.
  //
  // The PTY is not held open for the window either — a closed session's terminal
  // is gone. The restored card comes back unspawned, so the resume prompt is
  // what brings the conversation with it, exactly as for a parked worktree.
  const restoreClosedSession = useCallback(
    (projectId: string, session: Session, index: number, nextSeq: number) => {
      const next = restoreSession(sessionsRef.current, projectId, session, index)
      if (next === sessionsRef.current) {
        return // the project was closed while the toast was up
      }
      setSessions(next)
      const order = sessionsOf(next, projectId).map((s) => s.id)
      const { id, label, kind, path = "", providerSessionId } = session
      void Store.AddSession(projectId, id, label, kind, path, nextSeq)
        .then(() => providerSessionId && Store.SetProviderSession(id, providerSessionId))
        // AddSession appends: the slot the card went back to is a reorder.
        .then(() => Store.ReorderSessions(projectId, order))
    },
    [],
  )

  const closeSession = useCallback(
    (projectId: string, sessionId: string) => {
      const project = sessionsRef.current[projectId]
      const index = project?.sessions.findIndex((s) => s.id === sessionId) ?? -1
      if (!project || index === -1) {
        return
      }
      const session = project.sessions[index]
      const { nextSeq } = project
      // The conversation id lives on the row, not on the card — a session started
      // in this run never mirrors it into the page — so it is read back before
      // the delete takes it with it: an undo without it restores an empty card.
      const conversation = session.providerSessionId
        ? Promise.resolve(session.providerSessionId)
        : Store.ProviderSession(sessionId).catch(() => "")
      const deleted = dropSession(projectId, sessionId, (activeID) =>
        conversation.then(() => Store.DeleteSession(projectId, sessionId, activeID)),
      )
      toast(`Closed ${session.label}`, {
        duration: UNDO_TOAST_MS,
        action: {
          label: "Undo",
          // Waits on the delete: the store can sit on a lock for seconds, and an
          // insert that overtook it would be deleted right back out.
          onClick: () =>
            void Promise.all([conversation, deleted]).then(([providerSessionId]) =>
              restoreClosedSession(
                projectId,
                { ...session, ...(providerSessionId ? { providerSessionId } : {}) },
                index,
                nextSeq,
              ),
            ),
        },
      })
    },
    [dropSession, restoreClosedSession],
  )

  const discardSession = useCallback(
    (projectId: string, sessionId: string) => {
      void dropSession(projectId, sessionId, (activeID) =>
        Store.DeleteSession(projectId, sessionId, activeID),
      )
    },
    [dropSession],
  )

  const keepSession = useCallback(
    (projectId: string, sessionId: string) => {
      void dropSession(projectId, sessionId, (activeID) =>
        Store.CloseSession(projectId, sessionId, activeID),
      )
    },
    [dropSession],
  )

  const activateSession = useCallback((projectId: string, sessionId: string) => {
    const next = setActiveSession(sessionsRef.current, projectId, sessionId)
    if (next === sessionsRef.current) {
      return
    }
    setSessions(next)
    void Store.SetActiveSession(projectId, sessionId)
  }, [])

  // The session shortcuts act on the active project: one opens a session
  // (mirroring the "+" button), the other two walk its sidebar list. They fire
  // even with terminal focus, so they stay reachable while working in a terminal
  // — the capture-phase listener sees the chord before the terminal does and
  // stops propagation so the PTY never receives it. Bails while a hotkey is
  // being recorded so rebinding does not trigger it.
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
        return
      }
      const step = matchesCombo(event, hotkeys.nextSession)
        ? 1
        : matchesCombo(event, hotkeys.prevSession)
          ? -1
          : 0
      if (step === 0) {
        return
      }
      // Swallowed even with nowhere to go: the chord belongs to lich, so a
      // project with a single session must not leak it into that session's PTY.
      event.preventDefault()
      event.stopPropagation()
      const current = sessionsRef.current
      const target = neighborSessionId(
        current,
        activeProjectId,
        activeSessionId(current, activeProjectId),
        step,
      )
      if (target) {
        activateSession(activeProjectId, target)
      }
    }
    window.addEventListener("keydown", onKey, true)
    return () => window.removeEventListener("keydown", onKey, true)
  }, [activeProjectId, newSession, activateSession, hotkeys])

  // A request whose target worked and then answered somewhere lich cannot read
  // — its provider's own peer channel, or simply out loud to whoever is watching
  // — leaves the answer on that session's screen and nowhere else. The toast is
  // the only thing that says where it went, so it carries the way there.
  //
  // Meant to be rare: unifying the two names a session answers to, and telling
  // the target its ticket is the only route home, are what keep it rare. This is
  // the escape hatch for when neither held.
  useEffect(() => {
    const off = onAppEvent(RELAY_STALLED_EVENT, (data) => {
      if (!isRelayStalledEvent(data)) {
        return
      }
      const { targetId, target } = data
      const projectId = projectOfSession(sessionsRef.current, targetId)
      // The target's project was closed meanwhile: there is no card to open, so
      // the toast would offer a door to nowhere.
      if (!sessionsRef.current[projectId]) {
        return
      }
      toast(
        <div className="flex min-w-0 flex-col">
          <span>{target || UNLABELED_SESSION} answered in its own session</span>
          <span className="mt-0.5 text-xs text-muted-foreground">
            It finished without replying through lich — open it to read the answer.
          </span>
        </div>,
        {
          duration: ATTENTION_TOAST_MS,
          icon: <MessageSquareDashed className="size-4 text-sky-500" />,
          action: {
            label: "Open",
            onClick: () => {
              navigate(`/projects/${projectId}`)
              activateSession(projectId, targetId)
            },
          },
        },
      )
    })
    return () => off()
  }, [navigate, activateSession])

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
      if (!isStatusEvent(data)) {
        return
      }
      const { id } = data
      const status = toSessionStatus(data.state)
      const previous = lastStatusRef.current.get(id) ?? null
      lastStatusRef.current.set(id, status)

      const projectId = projectOfSession(sessionsRef.current, id)
      const project = sessionsRef.current[projectId]
      // A session whose project is closed — or whose own row is gone, which
      // leaves projectOfSession with nothing to answer — has nowhere to route.
      if (!project) {
        return
      }
      const label = project.sessions.find((s) => s.id === id)?.label ?? UNLABELED_SESSION
      const projectName = projectsRef.current.find((p) => p.id === projectId)?.name

      // The desktop channel answers to window focus and to its own per-status
      // preference, both decided by decideStatusNotice; the toast below keeps
      // its own, unchanged rule. The two are independent: a report can raise
      // both, either, or neither.
      const notice = decideStatusNotice(status, previous, document.hasFocus(), {
        attention: desktopNotificationsRef.current,
        finishedTurn: finishedTurnNotificationsRef.current,
      })
      if (notice === "ask") {
        setAskNotifications(true)
      } else if (notice === "notify") {
        const summary =
          status === "waiting" ? `${label} needs your input` : `${label} has finished working`
        // A failure is the backend's to log; the page has nothing to do with it.
        System.Notify(summary, projectName ?? "").catch(() => {})
      }

      if (
        status !== "waiting" ||
        !shouldToastAttention(sessionsRef.current, id, activeProjectIdRef.current)
      ) {
        return
      }
      toast(
        <div className="flex min-w-0 flex-col">
          <span>{label} needs your input</span>
          {projectName && (
            <span className="mt-0.5 flex items-center gap-1 text-xs text-muted-foreground">
              <Folder className="size-3 shrink-0" />
              <span className="truncate">{projectName}</span>
            </span>
          )}
        </div>,
        {
          duration: ATTENTION_TOAST_MS,
          icon: <Bell className="size-4 text-amber-500" />,
          action: {
            label: "Open",
            onClick: () => {
              navigate(`/projects/${projectId}`)
              activateSession(projectId, id)
            },
          },
        },
      )
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
      const session = sessionsRef.current[projectId]?.sessions.find((s) => s.id === id)
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

  const renameSession = useCallback((projectId: string, sessionId: string, label: string) => {
    const next = relabelSession(sessionsRef.current, projectId, sessionId, label)
    if (next === sessionsRef.current) {
      return
    }
    setSessions(next)
    void Store.RenameSession(sessionId, label)
  }, [])

  const pinSession = useCallback((projectId: string, sessionId: string, pinned: boolean) => {
    const next = setSessionPinned(sessionsRef.current, projectId, sessionId, pinned)
    if (next === sessionsRef.current) {
      return
    }
    setSessions(next)
    void Store.SetSessionPinned(sessionId, pinned)
  }, [])

  const value = useMemo(
    () => ({
      projects,
      sessions,
      homeId,
      openProject,
      openRecent,
      ensureHomeProject,
      closeProject,
      newSession,
      newWorktreeSession,
      reopenWorktreeSession,
      closeSession,
      discardSession,
      keepSession,
      activateSession,
      renameSession,
      pinSession,
      reorderProjects,
      reorderSessions,
    }),
    [
      projects,
      sessions,
      homeId,
      openProject,
      openRecent,
      ensureHomeProject,
      closeProject,
      newSession,
      newWorktreeSession,
      reopenWorktreeSession,
      closeSession,
      discardSession,
      keepSession,
      activateSession,
      renameSession,
      pinSession,
      reorderProjects,
      reorderSessions,
    ],
  )

  return (
    <ProjectsContext.Provider value={value}>
      {children}
      <NotificationsOptIn
        open={askNotifications}
        onDismiss={() => setAskNotifications(false)}
        onDecide={(enabled) => {
          setDesktopNotifications(enabled)
          setAskNotifications(false)
        }}
      />
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
