import type { Project, StoredProject } from "@/lib/api-types"
import { Store } from "@/lib/rpc"
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
      ...(session.run ? { run: true } : {}),
      ...(session.sandbox === "on" ? { sandboxed: true } : {}),
      ...(session.pinned ? { pinned: true } : {}),
      ...(session.folder ? { folder: session.folder } : {}),
      ...(session.originSessionId
        ? { originSessionId: session.originSessionId, originLabel: session.originLabel }
        : {}),
      ...(session.scheduledAt
        ? { scheduledAt: session.scheduledAt, scheduledPrompt: session.scheduledPrompt }
        : {}),
      ...(session.hasLastTurn ? { hasLastTurn: true } : {}),
      ...(session.mcpServers?.length ? { mcpServers: session.mcpServers } : {}),
      ...(session.sandboxSkippedLinks?.length
        ? { sandboxSkippedLinks: session.sandboxSkippedLinks }
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

// fileAfterInsert files a new session once `inserted`, its row's insert, has
// landed. Filing is an UPDATE, and one that reaches the store before the row does
// matches nothing, so the session would come back unfiled on the next load.
export async function fileAfterInsert(
  inserted: Promise<unknown>,
  sessionId: string,
  folder: string,
): Promise<void> {
  await inserted
  if (folder) {
    await Store.SetSessionFolder(sessionId, folder)
  }
}
