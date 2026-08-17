import type { Project, StoredProject } from "@/lib/api-types"
import { isSessionKind, type SessionState } from "@/lib/session/sessions"

export function toProject(project: StoredProject): Project {
  return { id: project.id, name: project.name, path: project.path }
}

export function buildSessionState(loaded: StoredProject[]): SessionState {
  const state: SessionState = {}
  for (const project of loaded) {
    const sessions = (project.sessions ?? []).map((session) => ({
      id: session.id,
      label: session.label,
      kind: isSessionKind(session.kind) ? session.kind : "claude",
      ...(session.path ? { path: session.path } : {}),
      ...(session.providerSessionId ? { providerSessionId: session.providerSessionId } : {}),
      ...(session.entrypoint ? { entrypoint: session.entrypoint } : {}),
      ...(session.pinned ? { pinned: true } : {}),
      ...(session.originSessionId
        ? { originSessionId: session.originSessionId, originLabel: session.originLabel }
        : {}),
    }))
    state[project.id] = {
      sessions,
      activeId: project.activeSessionId || sessions[0]?.id || "",
      nextSeq: project.nextSeq,
    }
  }
  return state
}
