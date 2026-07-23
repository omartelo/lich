import { useEffect, useRef, useState } from "react"
import { useMatch } from "react-router-dom"
import { TerminalView } from "@/components/TerminalView"
import { ResumeSessionDialog } from "@/components/ResumeSessionDialog"
import { useProjects } from "@/lib/projects"
import { activeSessionId, resumableSession, sessionsOf } from "@/lib/sessions"
import type { Session } from "@/lib/sessions"

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
// mounted only once the user has said whether to continue that conversation.
export function TerminalHost() {
  const { projects, sessions } = useProjects()
  const match = useMatch("/projects/:projectId")
  const activeProjectId = match?.params.projectId ?? null

  const visibleSessionId = activeProjectId
    ? activeSessionId(sessions, activeProjectId)
    : ""

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

  useEffect(() => {
    if (
      !activeProjectId ||
      !visibleSessionId ||
      spawnedRef.current.has(visibleSessionId)
    ) {
      return
    }
    const resumable = resumableSession(
      sessionsRef.current,
      activeProjectId,
      visibleSessionId,
    )
    if (resumable) {
      setAsking(resumable)
      return
    }
    setSpawned((prev) => new Set(prev).add(visibleSessionId))
  }, [activeProjectId, visibleSessionId])

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
          const visible =
            project.id === activeProjectId && session.id === projectActiveId
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
        onResume={() =>
          asking && answerResume(asking, asking.providerSessionId ?? "")
        }
      />
    </>
  )
}
