import { useEffect, useState } from "react"
import { useMatch } from "react-router-dom"
import { TerminalView } from "@/components/TerminalView"
import { XtermTerminalView } from "@/components/XtermTerminalView"
import { useProjects } from "@/lib/projects"
import { activeSessionId, sessionsOf } from "@/lib/sessions"
import { useGhosttyTerminal } from "@/lib/terminal-choice"

// Evaluated once at load: xterm.js + WebGL is the terminal; the legacy
// ghostty-web pipeline stays reachable via lib/terminal-choice.ts until
// phase 5 of docs/chromium-shell.md deletes it.
const TerminalComponent = useGhosttyTerminal(
  typeof localStorage !== "undefined" ? localStorage : null,
)
  ? TerminalView
  : XtermTerminalView

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
  useEffect(() => {
    if (!visibleSessionId) {
      return
    }
    setSpawned((prev) =>
      prev.has(visibleSessionId) ? prev : new Set(prev).add(visibleSessionId),
    )
  }, [visibleSessionId])

  return (
    <>
      {projects.flatMap((project) => {
        const projectActiveId = activeSessionId(sessions, project.id)
        return sessionsOf(sessions, project.id).map((session) => {
          if (!spawned.has(session.id)) {
            return null // lazy: not viewed yet, no PTY spawned
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
              <TerminalComponent
                sessionId={session.id}
                projectId={project.id}
                cwd={session.path || project.path}
                kind={session.kind}
                visible={visible}
              />
            </div>
          )
        })
      })}
    </>
  )
}
