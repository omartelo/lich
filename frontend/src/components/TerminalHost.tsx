import { useEffect, useMemo, useRef, useState } from "react"
import type { PointerEvent as ReactPointerEvent } from "react"
import { useMatch } from "react-router-dom"
import { toast } from "sonner"
import { TerminalView } from "./TerminalView"
import { ResumeSessionDialog } from "./ResumeSessionDialog"
import { ProviderIcon } from "./ProviderIcon"
import { CloseButton } from "./common/CloseButton"
import { WorktreeCloseDialogs } from "./sidebar/WorktreeCloseDialogs"
import { useWorktreeClose } from "./sidebar/useWorktreeClose"
import { Terminal as TerminalService } from "@/lib/rpc"
import { useProjects } from "@/providers/projects"
import { activeSessionId, hasSession, resumableSession, sessionsOf } from "@/lib/session/sessions"
import { paletteSessions } from "@/lib/session/command-palette"
import { spawnDecision, type SpawnProbe } from "@/lib/session/spawn-gate"
import { dragRatio, other, type Side } from "@/lib/session/panes"
import { closeBeside, setPaneRatio, usePaneRatio } from "@/lib/session/panes-store"
import { usePanes } from "@/lib/session/use-panes"
import { cn } from "@/lib/utils"
import type { Session } from "@/lib/session/sessions"

// The gate's two backend checks. Module-level so the effect below never takes a
// new object as a reason to run again.
const probe: SpawnProbe = {
  workdirMissing: TerminalService.WorkdirMissing,
  resumeAvailable: TerminalService.ResumeAvailable,
}

// Where a layer sits on the stage. Percentages rather than a flex row because
// every session is an absolutely positioned layer stacked in the same area —
// splitting moves two of them into their halves and leaves the rest alone.
type Lane = "full" | Side

// The ratio is the seam's distance from the left edge, so the left lane owns it
// and the right lane starts there — whichever session is drawing in each.
function laneStyle(lane: Lane, ratio: number): { left: string; width?: string; right?: string } {
  if (lane === "left") {
    return { left: "0", width: `${ratio * 100}%` }
  }
  if (lane === "right") {
    return { left: `${ratio * 100}%`, right: "0" }
  }
  return { left: "0", right: "0" }
}

// TerminalHost keeps one persistent terminal per session, across every open
// project, stacked in the same area. The router picks the active project and the
// per-project active session decides which layer is visible — terminals are
// never unmounted by navigation, so background sessions keep running. Inactive
// layers use visibility:hidden (not display:none) so they retain layout size and
// fit() stays correct.
//
// Sessions spawn lazily: a session's terminal (and its PTY) is created only once
// the session has first been viewed, not when its project loads. This keeps a
// restore of many projects × sessions from spawning every PTY at launch. Once
// spawned, a session stays mounted and running in the background.
//
// A restored session that ran Claude Code before the last restart carries that
// Claude session's id, so its spawn waits on the resume prompt: the terminal is
// mounted only once the user has said whether to continue that conversation —
// and only when there is still a conversation to continue (ResumeAvailable).
export function TerminalHost() {
  const { projects, sessions, keepSession } = useProjects()
  const match = useMatch("/projects/:projectId")
  const activeProjectId = match?.params.projectId ?? null

  const visibleSessionId = activeProjectId ? activeSessionId(sessions, activeProjectId) : ""

  // The second pane. Focus is not a third piece of state: the focused pane *is*
  // the project's active session, so the footer, the dock, the sidebar highlight
  // and every card shortcut keep reading activeId and needed no change here.
  const {
    beside: besideSessionId,
    besideSide,
    split,
    focusOther,
    promoteOther,
  } = usePanes(activeProjectId ?? "")
  const ratio = usePaneRatio()

  // The seam's ratio is held in state while dragging and written on release —
  // the panel-resize precedent (use-panel-width.ts), which keeps a drag from
  // putting a hundred writes through localStorage.
  const [dragging, setDragging] = useState<number | null>(null)
  const seamRef = useRef<{ startX: number; startRatio: number; width: number } | null>(null)
  const paneRatio = dragging ?? ratio

  const onSeamDown = (event: ReactPointerEvent<HTMLDivElement>) => {
    event.preventDefault()
    seamRef.current = {
      startX: event.clientX,
      startRatio: ratio,
      width: event.currentTarget.parentElement?.clientWidth ?? 0,
    }
    event.currentTarget.setPointerCapture(event.pointerId)
  }

  const seamAt = (clientX: number): number | null => {
    const drag = seamRef.current
    return drag ? dragRatio(drag.startRatio, drag.startX, clientX, drag.width) : null
  }

  const onSeamMove = (event: ReactPointerEvent<HTMLDivElement>) => {
    const next = seamAt(event.clientX)
    if (next !== null) {
      setDragging(next)
    }
  }

  const onSeamUp = (event: ReactPointerEvent<HTMLDivElement>) => {
    const next = seamAt(event.clientX)
    if (next === null) {
      return
    }
    seamRef.current = null
    event.currentTarget.releasePointerCapture(event.pointerId)
    setDragging(null)
    setPaneRatio(next)
  }

  // The close an exited session's banner raises, on its own instance of the
  // sidebar's flow — the sidebar is not always mounted (collapsed to the rail)
  // and this host always is. Bound to the active project, which is the only one
  // a click can reach: every other layer is visibility:hidden.
  const activeProjectPath = projects.find((p) => p.id === activeProjectId)?.path ?? ""
  const worktreeClose = useWorktreeClose(
    activeProjectId ?? "",
    activeProjectPath,
    sessionsOf(sessions, activeProjectId ?? ""),
  )

  // Session ids that have been viewed at least once, which keeps a terminal
  // mounted after the user navigates away. A session that leaves the workspace
  // is pruned from it (see below), not left behind as a dead entry.
  const [spawned, setSpawned] = useState<Set<string>>(() => new Set())
  // The session whose resume prompt is on screen, if any; its spawn waits here.
  const [asking, setAsking] = useState<Session | null>(null)
  // The Claude session each spawned session was told to resume, keyed by session
  // id. Only the ones the user accepted land here; everything else spawns fresh.
  const [resuming, setResuming] = useState<Record<string, string>>({})

  // Read by the effect below without being dependencies of it: the decision is
  // taken once per session id, and re-running it on an unrelated session or
  // spawn change would re-prompt a session already answered.
  const sessionsRef = useRef(sessions)
  sessionsRef.current = sessions
  const spawnedRef = useRef(spawned)
  spawnedRef.current = spawned
  const projectsRef = useRef(projects)
  projectsRef.current = projects

  // Which session the gate below is working on. Both panes spawn lazily and the
  // gate is written for one session at a time — its resume prompt is a single
  // dialog — so they are taken in order: the second pane's turn comes once the
  // first has landed in `spawned` and this recomputes.
  const pending = [visibleSessionId, besideSessionId].find((id) => id && !spawned.has(id)) ?? ""

  useEffect(() => {
    if (!activeProjectId || !pending) {
      return
    }
    const projectId = activeProjectId
    const sessionId = pending
    const session = sessionsOf(sessionsRef.current, projectId).find((s) => s.id === sessionId)
    const project = projectsRef.current.find((p) => p.id === projectId)
    const resumable = resumableSession(sessionsRef.current, projectId, sessionId)

    let live = true
    void spawnDecision(session?.path || project?.path || "", resumable, probe).then((decision) => {
      if (!live) {
        return
      }
      // Parking, not deleting: the checkout may come back (a re-created
      // worktree, a mount that was not up yet), and the row still carries the
      // provider conversation that a resume would need.
      if (decision.verdict === "park") {
        toast(decision.notice)
        keepSession(projectId, sessionId)
        return
      }
      if (decision.verdict === "ask" && resumable) {
        setAsking(resumable)
        return
      }
      if (decision.verdict === "fresh") {
        toast(decision.notice)
      }
      setSpawned((prev) => new Set(prev).add(sessionId))
    })
    return () => {
      live = false
    }
  }, [activeProjectId, pending, keepSession])

  // A session that left the workspace leaves both maps with it. Its id can come
  // back — an undone close restores the very same one — and its terminal did
  // not: the unmount closed the PTY. A leftover "spawned" entry would mount the
  // card straight into a fresh PTY, past the gate that asks about the
  // conversation, and a leftover resume id would then be answered for the user.
  useEffect(() => {
    const live = new Set(Object.values(sessions).flatMap((p) => p.sessions.map((s) => s.id)))
    setSpawned((prev) => {
      const kept = [...prev].filter((id) => live.has(id))
      return kept.length === prev.size ? prev : new Set(kept)
    })
    setResuming((prev) => {
      const kept = Object.entries(prev).filter(([id]) => live.has(id))
      return kept.length === Object.keys(prev).length ? prev : Object.fromEntries(kept)
    })
  }, [sessions])

  // The whole roster, for the session links each terminal draws in its own
  // output. Flattened once here rather than inside every TerminalView: this
  // host mounts one per spawned session across every project, so a per-view
  // join would walk the same workspace N times on every session change.
  const roster = useMemo(() => paletteSessions(projects, sessions), [projects, sessions])

  // Answer the prompt and release the spawn: resume is the Claude session id to
  // continue, or "" to start fresh.
  const answerResume = (session: Session, resume: string) => {
    setAsking(null)
    if (resume) {
      setResuming((prev) => ({ ...prev, [session.id]: resume }))
    }
    setSpawned((prev) => new Set(prev).add(session.id))
  }

  return (
    <>
      {projects.flatMap((project) => {
        const projectActiveId = activeSessionId(sessions, project.id)
        return sessionsOf(sessions, project.id).map((session) => {
          if (!spawned.has(session.id)) {
            return null
          }
          const onStage = project.id === activeProjectId
          const focused = onStage && session.id === projectActiveId
          const beside = onStage && session.id === besideSessionId
          const visible = focused || beside
          // A hidden layer keeps the whole stage: its terminal is destroyed
          // while hidden and refitted on the way back, so the size it holds
          // meanwhile is the one it would have unsplit — and nothing measures it.
          const lane: Lane = !split || !visible ? "full" : beside ? besideSide : other(besideSide)
          return (
            <div
              key={session.id}
              className={cn(
                "absolute inset-y-0 flex flex-col",
                lane === "right" && "border-l border-border",
              )}
              style={{ ...laneStyle(lane, paneRatio), visibility: visible ? "visible" : "hidden" }}
              aria-hidden={!visible}
              // Capture, so a click lands on the pane before xterm takes it for
              // a selection: clicking a terminal is how you focus its pane.
              onPointerDownCapture={beside ? focusOther : undefined}
            >
              {split && visible && (
                <div
                  className={cn(
                    "group flex shrink-0 items-center gap-1.5 px-2.5 pb-1 pt-1.5 text-xs",
                    focused ? "text-foreground" : "text-muted-foreground",
                  )}
                >
                  <ProviderIcon kind={session.kind} size={12} />
                  <span className="min-w-0 truncate font-medium">{session.label}</span>
                  <CloseButton
                    label={`Stop showing ${session.label}`}
                    className="ml-auto"
                    onClick={() => (focused ? promoteOther() : closeBeside(activeProjectId ?? ""))}
                  />
                </div>
              )}
              <div className="relative min-h-0 flex-1">
                <TerminalView
                  sessionId={session.id}
                  projectId={project.id}
                  cwd={session.path || project.path}
                  kind={session.kind}
                  resume={resuming[session.id] ?? ""}
                  roster={roster}
                  visible={visible}
                  focused={focused}
                  sandboxed={session.sandboxed ?? false}
                  onClose={() => worktreeClose.requestClose(session)}
                  stillInWorkspace={() => hasSession(sessionsRef.current, session.id)}
                />
              </div>
            </div>
          )
        })
      })}
      {split && (
        <div
          role="separator"
          aria-orientation="vertical"
          aria-label="Resize the panes"
          className="absolute inset-y-0 z-10 w-1.5 -translate-x-1/2 cursor-col-resize touch-none transition-colors hover:bg-accent"
          style={{ left: `${paneRatio * 100}%` }}
          onPointerDown={onSeamDown}
          onPointerMove={onSeamMove}
          onPointerUp={onSeamUp}
        />
      )}
      <ResumeSessionDialog
        session={asking}
        onStartNew={() => asking && answerResume(asking, "")}
        onResume={() => asking && answerResume(asking, asking.providerSessionId ?? "")}
      />
      <WorktreeCloseDialogs close={worktreeClose} />
    </>
  )
}
