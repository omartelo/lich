import { useEffect, useMemo, useRef, useState } from "react"
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
import { PaneSeams } from "./PaneSeams"
import { cellAt, grid, offsetOf, rowLength, rowTracks, tracks } from "@/lib/session/pane-grid"
import { usePanes } from "@/lib/session/use-panes"
import { useStageSize } from "@/lib/session/use-stage-size"
import { cn } from "@/lib/utils"
import type { Session } from "@/lib/session/sessions"

// The gate's two backend checks. Module-level so the effect below never takes a
// new object as a reason to run again.
const probe: SpawnProbe = {
  workdirMissing: TerminalService.WorkdirMissing,
  resumeAvailable: TerminalService.ResumeAvailable,
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

  // The stage: an ordered list of sessions, the grid computed from how many
  // there are and how wide the window is, and the cursor in one of the cells.
  // The focused cell always draws the project's active session, so the footer,
  // the dock, the sidebar highlight and every card shortcut go on reading
  // activeId and none of them needed a line.
  const stage = usePanes(activeProjectId ?? "")
  const [stageRef, size] = useStageSize()
  // Until the stage has been measured there is no grid to lay out — the first
  // paint of a window that opens onto a split would otherwise stack the panes
  // for one frame and then jump. Unmeasured draws the focused session alone,
  // which is what the stage looked like before any of this.
  const measured = size.width > 0
  const split = measured && stage.split
  const layout = grid(measured ? stage.cells.length : 1, size.width)

  // Track sizes come from the store, except while a seam is being dragged: the
  // panes have to move with the pointer, and only the release is written down.
  const [liveCols, setLiveCols] = useState<number[] | null>(null)
  const [liveRows, setLiveRows] = useState<number[] | null>(null)
  // The shares belong to the wall being drawn, not to the window: arranging one
  // group 60/40 must not carry into the next one the user opens.
  const cols = liveCols ?? tracks(stage.current?.cols ?? [], layout.cols)
  const rows = liveRows ?? tracks(stage.current?.rows ?? [], layout.rows)

  // The pane a dragged one is hovering over, for the drop hint. The drag itself
  // is a pointer gesture on the label rather than HTML5 drag-and-drop, which the
  // window already spends on files dropped into a terminal.
  const [dragFrom, setDragFrom] = useState<number | null>(null)
  const [dragOver, setDragOver] = useState<number | null>(null)

  const paneUnder = (x: number, y: number): number | null => {
    const el = document.elementFromPoint(x, y)?.closest("[data-pane]")
    const index = Number(el?.getAttribute("data-pane"))
    return Number.isInteger(index) ? index : null
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

  // Which session the gate below is working on. Every pane spawns lazily and the
  // gate is written for one session at a time — its resume prompt is a single
  // dialog — so the stage is taken in order: the next cell's turn comes once the
  // one before it has landed in `spawned` and this recomputes.
  const pending = stage.cells.find((id) => id && !spawned.has(id)) ?? ""

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
    <div ref={stageRef} className="absolute inset-0">
      {projects.flatMap((project) => {
        const projectActiveId = activeSessionId(sessions, project.id)
        return sessionsOf(sessions, project.id).map((session) => {
          if (!spawned.has(session.id)) {
            return null
          }
          const onStage = project.id === activeProjectId
          const index = onStage ? stage.cells.indexOf(session.id) : -1
          const focused = index >= 0 && session.id === projectActiveId
          const visible = focused || (measured && index >= 0)
          // A hidden layer keeps the whole stage: its terminal is destroyed
          // while hidden and refitted on the way back, so the size it holds
          // meanwhile is not a size anything measures.
          const place = cellAt(Math.max(index, 0), layout)
          const row = rowTracks(cols, rowLength(place.row, stage.cells.length, layout))
          const box = visible
            ? {
                left: `${offsetOf(row, place.col) * 100}%`,
                width: `${(row[place.col] ?? 1) * 100}%`,
                top: `${offsetOf(rows, place.row) * 100}%`,
                height: `${(rows[place.row] ?? 1) * 100}%`,
              }
            : { left: "0", right: "0", top: "0", bottom: "0" }
          return (
            <div
              key={session.id}
              data-pane={visible ? index : undefined}
              className={cn(
                "absolute flex flex-col",
                // Seams between panes are the one place DESIGN.md allows a
                // border: a hairline on every cell that has a neighbour behind
                // it, so the grid reads as divided rather than as boxes.
                visible && place.col > 0 && "border-l border-border",
                visible && place.row > 0 && "border-t border-border",
              )}
              style={{ ...box, visibility: visible ? "visible" : "hidden" }}
              aria-hidden={!visible}
              // Capture, so a click lands on the pane before xterm takes it for
              // a selection: clicking a terminal is how you focus its pane.
              onPointerDownCapture={visible && !focused ? () => stage.focusCell(index) : undefined}
            >
              {split && visible && (
                <div
                  className={cn(
                    "group flex shrink-0 cursor-grab items-center gap-1.5 px-2.5 pb-1 pt-1.5 text-xs",
                    focused ? "text-foreground" : "text-muted-foreground",
                    dragFrom === index && "cursor-grabbing opacity-60",
                    dragOver === index && dragFrom !== null && dragFrom !== index && "bg-accent/60",
                  )}
                  // The label is the grip: a pointer gesture on the terminal
                  // itself is a text selection, and one on the pane is how the
                  // cursor moves into it.
                  onPointerDown={(event) => {
                    event.currentTarget.setPointerCapture(event.pointerId)
                    setDragFrom(index)
                  }}
                  onPointerMove={(event) => {
                    if (dragFrom === index) {
                      setDragOver(paneUnder(event.clientX, event.clientY))
                    }
                  }}
                  onPointerUp={(event) => {
                    event.currentTarget.releasePointerCapture(event.pointerId)
                    const target = paneUnder(event.clientX, event.clientY)
                    setDragFrom(null)
                    setDragOver(null)
                    if (target === null || target === index) {
                      stage.focusCell(index)
                      return
                    }
                    stage.swap(index, target)
                  }}
                >
                  <ProviderIcon kind={session.kind} size={12} />
                  <span className="min-w-0 truncate font-medium">{session.label}</span>
                  <CloseButton
                    label={`Stop showing ${session.label}`}
                    className="ml-auto"
                    onClick={() => stage.drop(index)}
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
      {/* Column seams are drawn per row so they stop at the row they divide,
          and a short last row — which spreads its cells over the whole width —
          has none: the boundary it would offer is not one of the columns the
          stored track sizes describe. */}
      {split &&
        rows.map((_, row) =>
          rowLength(row, stage.cells.length, layout) === layout.cols ? (
            <PaneSeams
              // A row is its place in the grid, and the seams under it divide
              // that row and nothing else.
              // biome-ignore lint/suspicious/noArrayIndexKey: the index is the identity here
              key={`row-${row}`}
              tracks={cols}
              axis="cols"
              extent={size.width}
              from={offsetOf(rows, row)}
              span={rows[row]}
              onChange={setLiveCols}
              onCommit={(next) => {
                setLiveCols(null)
                if (stage.current) {
                  stage.setTracks(stage.current.id, { cols: next })
                }
              }}
            />
          ) : null,
        )}
      {split && (
        <PaneSeams
          tracks={rows}
          axis="rows"
          extent={size.height}
          onChange={setLiveRows}
          onCommit={(next) => {
            setLiveRows(null)
            if (stage.current) {
              stage.setTracks(stage.current.id, { rows: next })
            }
          }}
        />
      )}
      <ResumeSessionDialog
        session={asking}
        onStartNew={() => asking && answerResume(asking, "")}
        onResume={() => asking && answerResume(asking, asking.providerSessionId ?? "")}
      />
      <WorktreeCloseDialogs close={worktreeClose} />
    </div>
  )
}
