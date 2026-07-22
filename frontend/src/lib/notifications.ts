// The notification queue's join: raw pending statuses (session id + state) meet
// live project state to become routable rows. Kept pure (no React) so the queue
// logic is testable without the status-store singleton or a render.

import {projectOfSession, type SessionState} from "./sessions"
import type {PendingStatus} from "./session-status-store"

// A pending status resolved to the project and label the queue renders and
// routes to.
export interface Notification extends PendingStatus {
  projectId: string
  projectName: string
  sessionLabel: string
}

// Only the fields the join needs from a project — structural, so the provider's
// richer project object passes as-is.
interface ProjectRef {
  id: string
  name: string
}

// notificationsFrom resolves each pending status against the current projects
// and sessions, dropping two kinds:
//   - a status whose session no longer exists (a closed session leaves its last
//     state in the store) — it has nowhere to route;
//   - the focused session (the active session of the active project) — it is on
//     screen, so its own terminal already shows the state; queuing it is the bug
//     of notifying you about the very turn you are watching finish. This is the
//     queue's half of the rule the toast applies via shouldToastAttention.
export function notificationsFrom(
  pending: readonly PendingStatus[],
  projects: readonly ProjectRef[],
  sessions: SessionState,
  activeProjectId: string | undefined,
): Notification[] {
  const result: Notification[] = []
  for (const item of pending) {
    const projectId = projectOfSession(sessions, item.id)
    const project = projects.find((p) => p.id === projectId)
    const session = sessions[projectId]?.sessions.find((s) => s.id === item.id)
    if (!project || !session) {
      continue
    }
    const focused =
      projectId === activeProjectId && sessions[projectId].activeId === item.id
    if (focused) {
      continue
    }
    result.push({
      ...item,
      projectId,
      projectName: project.name,
      sessionLabel: session.label,
    })
  }
  return result
}
