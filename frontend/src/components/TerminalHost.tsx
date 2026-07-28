import { useEffect, useRef, useState } from "react"
import { useMatch } from "react-router-dom"
import { toast } from "sonner"
import { TerminalView } from "./TerminalView"
import { ResumeSessionDialog } from "./ResumeSessionDialog"
import { Terminal as TerminalService } from "@/lib/rpc"
import { useProjects } from "@/providers/projects"
import { activeSessionId, resumableSession, sessionsOf } from "@/lib/session/sessions"
import { spawnDecision, type SpawnProbe } from "@/lib/session/spawn-gate"
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

  const visibleSessionId = activeProjectId ? activeSessionId(sessions, activeProjectId) : ""

  // Session ids that have been viewed at least once. A viewed session stays in
  // the set (ids are unique uuids, so closed sessions leave only harmless dead
  // entries), which keeps its terminal mounted after the user navigates away.
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

  useEffect(() => {
    if (!activeProjectId || !visibleSessionId || spawnedRef.current.has(visibleSessionId)) {
      return
    }
    const projectId = activeProjectId
    const sessionId = visibleSessionId
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
  }, [activeProjectId, visibleSessionId, keepSession])

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
          const visible = project.id === activeProjectId && session.id === projectActiveId
          return (
            <div
              key={session.id}
              className="absolute inset-0"
              style={{ visibility: visible ? "visible" : "hidden" }}
              aria-hidden={!visible}
            >
              <TerminalView
                sessionId={session.id}
                projectId={project.id}
                cwd={session.path || project.path}
                kind={session.kind}
                resume={resuming[session.id] ?? ""}
                visible={visible}
              />
            </div>
          )
        })
      })}
      <ResumeSessionDialog
        session={asking}
        onStartNew={() => asking && answerResume(asking, "")}
        onResume={() => asking && answerResume(asking, asking.providerSessionId ?? "")}
      />
    </>
  )
}
