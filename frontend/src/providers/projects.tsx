import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import type { ReactNode } from "react"
import { toast } from "sonner"
import { Bell, Folder, MessageSquareDashed } from "lucide-react"
import { useMatch, useNavigate } from "react-router-dom"
import type { ClosedSession, Project, RecentProject } from "@/lib/api-types"
import type { StoredProject as StoreProject, StoredSession } from "@/lib/api-types"
import { ProjectService, Store, System } from "@/lib/rpc"
import { onAppEvent } from "@/lib/app-events"
import { storedGroups } from "@/lib/session/panes-store"
import {
  activeSessionId,
  addSession,
  adoptSession,
  closeSession as removeSession,
  dropClosedSession,
  isSessionKind,
  neighborSessionId,
  projectOfSession,
  removeProject,
  renameSession as relabelSession,
  reorderSessions as rearrangeSessions,
  restoreSession,
  sessionsOf,
  setActiveSession,
  setSessionEntrypoint as recordEntrypoint,
  setSessionPinned,
  setSessionSandboxed,
  type Session,
  type SessionKind,
  type SessionState,
} from "@/lib/session/sessions"
import { applyOrder, pinFirst } from "@/lib/reorder"
import { displayPath } from "@/lib/paths"
import { errorText } from "@/lib/utils"
import {
  hydrateProjectProviderDefaults,
  loadProviders,
  projectNewSessionKind,
} from "@/lib/providers-store"
import { resolveNewSessionKind } from "@/lib/session/new-session-kind"
import {
  CLOSED_EVENT,
  OPENED_EVENT,
  PROJECT_OPENED_EVENT,
  RELAY_STALLED_EVENT,
  SANDBOX_EVENT,
  STATUS_EVENT,
  TITLE_EVENT,
  TOUCHED_EVENT,
  decideStatusNotice,
  isIdEvent,
  isRelayStalledEvent,
  isSandboxEvent,
  isStatusEvent,
  isTitleEvent,
  shouldToastAttention,
  statusReason,
  toClosedSession,
  toOpenedProject,
  toOpenedSession,
  toSessionStatus,
  type SessionStatus,
} from "@/lib/session/session-events"
import { NotificationsOptIn } from "@/components/NotificationsOptIn"
import { refreshGitStatus } from "@/lib/git/use-git-status"
import { markSessionSeen } from "@/lib/session/use-session-status"
import { useHotkey } from "@/lib/use-hotkey"
import { neighborProjectId } from "@/lib/project-order"
import { requestTerminalFocus } from "@/lib/terminal/focus-request"
import { useSettings } from "./settings"
import { buildSessionState, toProject } from "./project-workspace"
import { ProjectsContext } from "./projects-context"

export { useProjects } from "./projects-context"

const newSessionId = (): string => crypto.randomUUID()

// cardFromStored turns the row a resume hands back into the card the sidebar
// draws. Shared by both doors into a resume — the worktree picker and the
// history list — so a field one carried and the other dropped cannot happen.
const cardFromStored = (restored: StoredSession): Session => ({
  id: restored.id,
  label: restored.label,
  kind: isSessionKind(restored.kind) ? restored.kind : "claude",
  path: restored.path,
  ...(restored.providerSessionId ? { providerSessionId: restored.providerSessionId } : {}),
  ...(restored.entrypoint ? { entrypoint: restored.entrypoint } : {}),
  ...(restored.sandbox === "on" ? { sandboxed: true } : {}),
  ...(restored.originSessionId
    ? { originSessionId: restored.originSessionId, originLabel: restored.originLabel }
    : {}),
})

// The first session of any project is always "Session 1"; the counter then
// points at 2 for the next one.
const FIRST_LABEL = "Session 1"
const FIRST_NEXT_SEQ = 2

const ATTENTION_TOAST_MS = 10_000

// How long the undo stays on offer after a close. Long enough to notice the card
// is gone and reach the button, short enough that it is not still there when the
// next one is closed on purpose.
const UNDO_TOAST_MS = 10_000

const UNLABELED_SESSION = "A session"

// ProjectsProvider is the write-through layer over the SQLite store: it mirrors
// every mutation to the store and hydrates from it on launch so open projects
// and their sessions survive restarts. In-project mutations read session state
// through sessionsRef and publish it through commit, which keeps the two in step
// without waiting for a render.
export function ProjectsProvider({ children }: { children: ReactNode }) {
  const [projects, setProjects] = useState<Project[]>([])
  const [sessions, setSessions] = useState<SessionState>({})
  const sessionsRef = useRef(sessions)
  sessionsRef.current = sessions
  // commit publishes new session state and advances the ref in the same breath.
  // Every mutation reads the ref, and React renders once per tick at the
  // earliest: two of them inside one tick — a worktree's occupants being closed
  // together, two backend events delivered back to back — would otherwise both
  // read the state from before the first, and the second would put back what the
  // first took away.
  const commit = useCallback((next: SessionState) => {
    sessionsRef.current = next
    setSessions(next)
  }, [])
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
    commit(buildSessionState(loaded))
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
      hydrateProjectProviderDefaults(loaded)
      applyLoaded(loaded)
      void loadProviders().catch(() => undefined)
    })()
  }, [applyLoaded])

  // A label changed outside the window: the auto-applied Claude ai-title (only
  // while the user has not renamed it), or `lich rename` and its MCP tool.
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
        commit(next)
      }
    })
    return () => off()
  }, [])

  // Every spawn reports whether its PTY runs confined. Mirrored into local
  // state so the card wears its mark the moment the session opens, rather than
  // at the next reload — the row is already written, so this never writes back.
  useEffect(() => {
    const off = onAppEvent(SANDBOX_EVENT, (data) => {
      if (!isSandboxEvent(data)) {
        return
      }
      const next = setSessionSandboxed(sessionsRef.current, data.id, data.confined)
      if (next !== sessionsRef.current) {
        commit(next)
      }
    })
    return () => off()
  }, [])

  // A project an agent opened through the CLI or its MCP tools by naming a
  // directory: the row is written, so nothing is persisted here — the tab is
  // drawn from the payload, which carries the sessions a project reopened from
  // the history comes back with.
  //
  // Nothing here waits on a reload, and that is load-bearing rather than an
  // optimisation: the session event that follows lands in this project, and a
  // card whose project is not on screen yet is dropped (adoptSession). Nor is it
  // navigated to — nobody in front of the window asked for this tab, and an
  // agent opening three would otherwise drag the view along three times.
  useEffect(() => {
    const off = onAppEvent(PROJECT_OPENED_EVENT, (data) => {
      const project = toOpenedProject(data)
      if (!project || projectsRef.current.some((p) => p.id === project.id)) {
        return
      }
      hydrateProjectProviderDefaults([project])
      setProjects((prev) => [...prev, toProject(project)])
      commit({ ...sessionsRef.current, ...buildSessionState([project]) })
    })
    return () => off()
  }, [])

  // A session an agent opened through the CLI or its MCP tools: the backend
  // wrote the row and started the PTY, so nothing is persisted or spawned here
  // — only the card is added, unfocused, to the project it belongs to.
  useEffect(() => {
    const off = onAppEvent(OPENED_EVENT, (data) => {
      const opened = toOpenedSession(data)
      if (!opened) {
        return
      }
      const { id, projectId, label, kind, path, nextSeq, originSessionId, originLabel } = opened
      const session: Session = {
        id,
        label,
        kind,
        ...(path ? { path } : {}),
        ...(originSessionId ? { originSessionId, originLabel } : {}),
      }
      const next = adoptSession(sessionsRef.current, projectId, session, nextSeq)
      if (next !== sessionsRef.current) {
        commit(next)
      }
    })
    return () => off()
  }, [])

  // A session an agent closed through the CLI or its MCP tools: the row is
  // already gone and its PTY with it, so only the card is taken down here.
  useEffect(() => {
    const off = onAppEvent(CLOSED_EVENT, (data) => {
      const closed = toClosedSession(data)
      if (!closed) {
        return
      }
      const next = dropClosedSession(
        sessionsRef.current,
        closed.projectId,
        closed.id,
        closed.activeId,
      )
      if (next !== sessionsRef.current) {
        commit(next)
      }
    })
    return () => off()
  }, [])

  // A reopened project keeps the sessions it was closed with, hence the reload.
  // Nothing is seeded: a brand-new project lands on the empty screen.
  const adopt = useCallback(
    async (project: Project) => {
      await Store.AddProject(project.id, project.name, project.path)
      const loaded = (await Store.LoadState()) ?? []
      hydrateProjectProviderDefaults(loaded)
      applyLoaded(loaded)
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
  // there. A project whose folder was moved or renamed is not dropped — the
  // picker asks where it went and the project is repointed under its own id,
  // which is what its sessions and its worktree directory hang off. Cancelling
  // leaves the row exactly as it was, to relocate on the next attempt; a
  // directory another project already holds is refused backend-side.
  //
  // Answers whether the project is open afterwards, for the caller that has more
  // to do once it is: resuming a session of a closed project reopens the project
  // first, and a cancelled relocate has to stop that resume rather than land a
  // card in a project the window is not holding.
  const openRecent = useCallback(
    async (recent: RecentProject): Promise<boolean> => {
      if (await ProjectService.Exists(recent.path)) {
        await adopt(recent)
        return true
      }
      try {
        const moved = await ProjectService.Relocate(recent.id)
        if (!moved) {
          return false
        }
        await adopt(moved)
        toast.success(`${moved.name} now opens ${displayPath(moved.path)}`)
        return true
      } catch (error) {
        toast.error(`Relocate failed: ${errorText(error)}`)
        return false
      }
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
      commit(removeProject(sessionsRef.current, id))
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

  const newSession = useCallback((projectId: string, kind?: SessionKind, path = "") => {
    const sessionId = newSessionId()
    const resolvedKind = resolveNewSessionKind(kind, projectNewSessionKind(projectId))
    const next = addSession(sessionsRef.current, projectId, sessionId, resolvedKind, path)
    const project = next[projectId]
    const created = project.sessions[project.sessions.length - 1]
    commit(next)
    void Store.AddSession(projectId, sessionId, created.label, resolvedKind, path, project.nextSeq)
    return sessionId
  }, [])

  // sandbox is the answer the new-worktree dialog collected: "on", "off", or ""
  // when the machine cannot confine anything and nothing was asked.
  //
  // from is the session this one is being forked out of, when it is: it decides
  // the provider the new card runs — a fork has to spawn the CLI that wrote the
  // conversation it carries, not whatever the project defaults to — and records
  // the lineage the sidebar already draws for a delegate ("from <parent>").
  const newWorktreeSession = useCallback(
    (
      projectId: string,
      wt: { name: string; path: string },
      sandbox = "",
      from: Session | null = null,
    ) => {
      const sessionId = newSessionId()
      const kind = from?.kind ?? projectNewSessionKind(projectId)
      const next = addSession(sessionsRef.current, projectId, sessionId, kind, wt.path, wt.name)
      const project = next[projectId]
      const created = project.sessions[project.sessions.length - 1]
      commit(next)
      if (from) {
        void Store.AddSessionFrom(
          projectId,
          sessionId,
          created.label,
          kind,
          wt.path,
          project.nextSeq,
          from.id,
          from.label,
        )
        return sessionId
      }
      void Store.AddSession(
        projectId,
        sessionId,
        created.label,
        kind,
        wt.path,
        project.nextSeq,
        sandbox,
      )
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
        return newWorktreeSession(projectId, wt)
      }
      commit(restoreSession(sessionsRef.current, projectId, cardFromStored(restored)))
      return restored.id
    },
    [newWorktreeSession],
  )

  // Resume one session picked out of the history, wherever it was parked. The
  // project comes back first when its tab is gone — a card cannot land in a
  // project the window is not holding — and a cancelled relocate stops the
  // resume rather than reopening the session into nowhere.
  //
  // A row the store no longer has is not an error: another window resumed it, or
  // its worktree was removed since the list was drawn. The toast says which,
  // because the row disappearing with no explanation reads as a failure.
  const resumeClosedSession = useCallback(
    async (closed: ClosedSession) => {
      if (!sessionsRef.current[closed.projectId]) {
        const opened = await openRecent({
          id: closed.projectId,
          name: closed.projectName,
          path: closed.projectPath,
        })
        if (!opened) {
          return
        }
      }
      const restored = await Store.ReopenSession(closed.id, newSessionId())
      if (!restored) {
        toast(`${closed.label} is no longer available to resume`)
        return
      }
      commit(restoreSession(sessionsRef.current, closed.projectId, cardFromStored(restored)))
      navigate(`/projects/${closed.projectId}`)
    },
    [openRecent, navigate],
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
      commit(removed)
      return persist(activeSessionId(removed, projectId))
    },
    [],
  )

  // Undo puts the parked row back rather than re-creating one: every close parks
  // now, so the reopen-by-id the history list needed is the same door an undo
  // wants, and re-inserting a second row for a session the store still holds
  // would leave the first one hidden in the history forever.
  //
  // What it therefore keeps that the old re-insert dropped: label_auto, the cost
  // ledgers, and the position — the resume appends and the reorder below puts the
  // card back in its slot.
  //
  // The PTY is not held open for the toast's lifetime — a closed session's
  // terminal is gone. The restored card comes back unspawned under a fresh id, so
  // the resume prompt is what brings the conversation with it, exactly as for a
  // parked worktree.
  const restoreClosedSession = useCallback(
    async (projectId: string, session: Session, index: number) => {
      const restored = await Store.ReopenSession(session.id, newSessionId())
      if (!restored) {
        toast.error(`Could not bring ${session.label} back`)
        return
      }
      const next = restoreSession(sessionsRef.current, projectId, cardFromStored(restored), index)
      if (next === sessionsRef.current) {
        return // the project was closed while the toast was up
      }
      commit(next)
      // The resume appends, so the slot the card went back to is a reorder.
      void Store.ReorderSessions(
        projectId,
        sessionsOf(next, projectId).map((s) => s.id),
      )
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
      // Parked, not deleted: the row is what the history lists and what an undo
      // resumes, and its conversation id, cost ledgers and chosen name all ride
      // on it. Removing the checkout is the one close that still deletes
      // (discardSession), because a row must not outlive its directory.
      const parked = dropSession(projectId, sessionId, (activeID) =>
        Store.CloseSession(projectId, sessionId, activeID),
      )
      toast(`Closed ${session.label}`, {
        duration: UNDO_TOAST_MS,
        action: {
          label: "Undo",
          // Waits on the park: the store can sit on a lock for seconds, and a
          // resume that overtook it would find the row still open and do nothing.
          onClick: () => void parked.then(() => restoreClosedSession(projectId, session, index)),
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

  // closeSession without the toast: the keep-or-remove dialog already asked, so
  // there is nothing left to offer an undo for.
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
    commit(next)
    void Store.SetActiveSession(projectId, sessionId)
  }, [])

  // The session shortcuts act on the active project: one opens a session
  // (mirroring the "+" button), two walk its sidebar list, and one puts the
  // cursor back in the active session's terminal. They fire even with terminal
  // focus — see useHotkey for how the chord is kept out of the PTY. A press with
  // no project open is declined, so the chord falls through instead of being
  // swallowed for nothing.
  useHotkey(hotkeys.newSession, () => {
    if (!activeProjectId) return false
    newSession(activeProjectId)
  })

  const stepSession = (step: 1 | -1) => {
    if (!activeProjectId) return false
    // Swallowed even with nowhere to go: the chord belongs to lich, so a project
    // with a single session must not leak it into that session's PTY.
    const current = sessionsRef.current
    const target = neighborSessionId(
      current,
      activeProjectId,
      activeSessionId(current, activeProjectId),
      step,
      // The walk follows what the sidebar draws, and the split's block is drawn
      // first: without it the step would jump a divider and come back.
      storedGroups(activeProjectId),
    )
    if (target) {
      activateSession(activeProjectId, target)
    }
  }
  useHotkey(hotkeys.nextSession, () => stepSession(1))
  useHotkey(hotkeys.prevSession, () => stepSession(-1))

  // Settings and the pull requests render over the terminals rather than beside
  // them, so handing focus to a session that is behind one of those screens would
  // type into something the user cannot see: leave the screen first.
  useHotkey(hotkeys.focusTerminal, () => {
    if (!activeProjectId) return false
    const target = activeSessionId(sessionsRef.current, activeProjectId)
    if (!target) return false
    navigate(`/projects/${activeProjectId}`)
    requestTerminalFocus(target)
  })

  const stepProject = (step: 1 | -1) => {
    const target = neighborProjectId(projectsRef.current, homeIdRef.current, activeProjectId, step)
    if (!target) return false
    navigate(`/projects/${target}`)
  }
  useHotkey(hotkeys.nextProject, () => stepProject(1))
  useHotkey(hotkeys.prevProject, () => stepProject(-1))

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
      // Read off the raw event like the status above, and for the same reason:
      // the store collapses a repeat "waiting", and a second prompt in one turn
      // is a second question to show.
      const reason = statusReason(data)

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
          {/* What it is blocked on, when the provider's event had words for it
              (docs/hooks/session-state.md). The toast is read from across the
              screen and its whole job is to say which card is worth the trip. */}
          {reason && (
            <span className="mt-0.5 truncate text-xs text-muted-foreground">{reason}</span>
          )}
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

  // The one session whose terminal is on screen, which is the only one the user
  // can be said to have read. Everything that answers "what came back while I
  // was away" hangs off it: the card's own ring, its project's tab badge and
  // the notification queue all ask the same question of the same mark.
  //
  // Three moments count as reading it: arriving at the card, a report landing
  // while it is on screen, and coming back to a window that was in the
  // background. The window's focus is what separates the last two from a card
  // left open in an app nobody is looking at — the same fact the desktop
  // notification answers to, and one only the page holds. The cleanup marks the
  // card being left, so a turn that finished while it was on screen does not
  // badge the tab on the way out.
  const focusedSessionId = activeProjectId ? activeSessionId(sessions, activeProjectId) : ""
  useEffect(() => {
    if (!focusedSessionId) {
      return
    }
    const markSeen = () => markSessionSeen(focusedSessionId)
    const markSeenIfWatched = () => {
      if (document.hasFocus()) {
        markSeen()
      }
    }
    markSeenIfWatched()
    const off = onAppEvent(STATUS_EVENT, (data) => {
      if (isIdEvent(data) && data.id === focusedSessionId) {
        markSeenIfWatched()
      }
    })
    window.addEventListener("focus", markSeen)
    return () => {
      off()
      window.removeEventListener("focus", markSeen)
      markSeen()
    }
  }, [focusedSessionId])

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
    commit(next)
    void Store.ReorderSessions(projectId, ids)
  }, [])

  const renameSession = useCallback((projectId: string, sessionId: string, label: string) => {
    const next = relabelSession(sessionsRef.current, projectId, sessionId, label)
    if (next === sessionsRef.current) {
      return
    }
    commit(next)
    void Store.RenameSession(sessionId, label)
  }, [])

  // setEntrypoint records the command a terminal opens into and reports back,
  // because saving it changes nothing the user can see: the running PTY keeps
  // whatever is in it, and the command only takes over on the next spawn. The
  // card is left to say the rest — its name once lich still owns the name, its
  // tooltip once the user has taken it over.
  const setEntrypoint = useCallback((projectId: string, sessionId: string, entrypoint: string) => {
    Store.SetSessionEntrypoint(sessionId, entrypoint)
      // The store answers whether the label actually moved; it refuses a card
      // the user has renamed, and that answer is what decides the card here.
      .then(() => (entrypoint ? Store.SetSessionTitle(sessionId, entrypoint) : false))
      .then((renamed) => {
        commit(recordEntrypoint(sessionsRef.current, projectId, sessionId, entrypoint, !!renamed))
        toast.success(entrypoint ? "Entrypoint set" : "Entrypoint cleared", {
          description: entrypoint
            ? "Runs the next time this terminal starts."
            : "This terminal starts a plain shell again.",
        })
      })
      .catch((error: unknown) => toast.error(`Could not save the entrypoint: ${errorText(error)}`))
  }, [])

  const pinSession = useCallback((projectId: string, sessionId: string, pinned: boolean) => {
    const next = setSessionPinned(sessionsRef.current, projectId, sessionId, pinned)
    if (next === sessionsRef.current) {
      return
    }
    commit(next)
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
      resumeClosedSession,
      closeSession,
      discardSession,
      keepSession,
      activateSession,
      renameSession,
      setEntrypoint,
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
      resumeClosedSession,
      closeSession,
      discardSession,
      keepSession,
      activateSession,
      renameSession,
      setEntrypoint,
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
