import { useEffect } from "react"
import type { Dispatch, MutableRefObject, SetStateAction } from "react"
import type { NavigateFunction } from "react-router-dom"
import { toast } from "sonner"
import { AlarmClockOff, Bell, Folder, MessageSquareDashed } from "lucide-react"
import type { Project } from "@/lib/api-types"
import { System } from "@/lib/rpc"
import { onAppEvent } from "@/lib/app-events"
import { hydrateProjectProviderDefaults } from "@/lib/providers-store"
import { refreshGitStatus } from "@/lib/git/use-git-status"
import { markSessionSeen } from "@/lib/session/use-session-status"
import { scheduledFor } from "@/lib/session/schedule"
import {
  activeSessionId,
  adoptSession,
  dropClosedSession,
  projectOfSession,
  renameSession as relabelSession,
  setSessionMCPServers,
  setSessionSandboxed,
  setSessionSchedule,
  type Session,
  type SessionState,
} from "@/lib/session/sessions"
import {
  CLOSED_EVENT,
  OPENED_EVENT,
  PROJECT_OPENED_EVENT,
  RELAY_STALLED_EVENT,
  MCP_EVENT,
  SANDBOX_EVENT,
  SCHEDULE_EVENT,
  SCHEDULE_FORFEIT_EVENT,
  STATUS_EVENT,
  TITLE_EVENT,
  TOUCHED_EVENT,
  decideStatusNotice,
  isForfeitedScheduleEvent,
  isIdEvent,
  isRelayStalledEvent,
  isMCPEvent,
  isSandboxEvent,
  isScheduleEvent,
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
import { buildSessionState, toProject } from "./project-workspace"

const ATTENTION_TOAST_MS = 10_000

// How long the notice of a lost scheduled prompt stays up. Longer than the rest:
// it carries the only remaining copy of what the user wrote, and once it goes
// there is nowhere left to read it.
const FORFEIT_TOAST_MS = 30_000

const UNLABELED_SESSION = "A session"

// What the subscriptions below reach for. Every ref is the provider's own, read
// inside once-only subscriptions rather than closed over as state: a listener
// torn down and rebuilt on every render would miss the events that land between
// the two.
export interface SessionEventDeps {
  sessions: SessionState
  activeProjectId: string | undefined
  sessionsRef: MutableRefObject<SessionState>
  projectsRef: MutableRefObject<Project[]>
  activeProjectIdRef: MutableRefObject<string | undefined>
  desktopNotificationsRef: MutableRefObject<boolean | null>
  finishedTurnNotificationsRef: MutableRefObject<boolean>
  lastStatusRef: MutableRefObject<Map<string, SessionStatus | null>>
  commit: (next: SessionState) => void
  setProjects: Dispatch<SetStateAction<Project[]>>
  setAskNotifications: Dispatch<SetStateAction<boolean>>
  activateSession: (projectId: string, sessionId: string) => void
  navigate: NavigateFunction
}

// useSessionEvents subscribes the provider to everything the backend announces
// about a session it did not itself ask for: a rename from the CLI, a spawn's
// sandbox and MCP reports, a scheduled prompt delivered or lost, a session or
// project an agent opened or closed, and the status reports that raise a toast,
// a desktop notification and the unread mark.
export function useSessionEvents({
  sessions,
  activeProjectId,
  sessionsRef,
  projectsRef,
  activeProjectIdRef,
  desktopNotificationsRef,
  finishedTurnNotificationsRef,
  lastStatusRef,
  commit,
  setProjects,
  setAskNotifications,
  activateSession,
  navigate,
}: SessionEventDeps): void {
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
      const next = setSessionSandboxed(
        sessionsRef.current,
        data.id,
        data.confined,
        data.skippedLinks ?? [],
      )
      if (next !== sessionsRef.current) {
        commit(next)
      }
    })
    return () => off()
  }, [])

  // Every spawn reports the MCP servers its provider could reach, so a card
  // divides a tool name against the list that spawn resolved rather than one
  // read when lich started. The row is already written, so this never writes
  // back.
  useEffect(() => {
    const off = onAppEvent(MCP_EVENT, (data) => {
      if (!isMCPEvent(data)) {
        return
      }
      const next = setSessionMCPServers(sessionsRef.current, data.id, data.servers)
      if (next !== sessionsRef.current) {
        commit(next)
      }
    })
    return () => off()
  }, [])

  // A scheduled prompt has been typed at its session, so the mark comes off the
  // card at that moment rather than at the next reload. The row is already
  // cleared on the backend's side of it, so this never writes back.
  useEffect(() => {
    const off = onAppEvent(SCHEDULE_EVENT, (data) => {
      if (!isScheduleEvent(data)) {
        return
      }
      const next = setSessionSchedule(sessionsRef.current, data.id, data.at, "")
      if (next !== sessionsRef.current) {
        commit(next)
      }
    })
    return () => off()
  }, [])

  // A prompt parked on a session that was deleted for good is gone with the row
  // — the row was the only copy of it. The toast is the whole of the amends: it
  // says which session it was parked on and what it said, so the words can be
  // read off the screen and parked somewhere else. No route to open, because
  // there is nothing left to open.
  useEffect(() => {
    const off = onAppEvent(SCHEDULE_FORFEIT_EVENT, (data) => {
      if (!isForfeitedScheduleEvent(data)) {
        return
      }
      toast(
        <div className="flex min-w-0 flex-col">
          <span>Scheduled prompt lost with {data.label}</span>
          {/* Clamped rather than cut: a prompt is capped at 8 KB, and the
              toast's job is to name it, not to hold the whole of it. */}
          <span className="mt-0.5 line-clamp-2 text-xs text-muted-foreground">
            Due {scheduledFor(data.at, new Date())}: {data.prompt}
          </span>
        </div>,
        { duration: FORFEIT_TOAST_MS, icon: <AlarmClockOff className="size-4 text-amber-500" /> },
      )
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
        ...(opened.run ? { run: true } : {}),
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
}
