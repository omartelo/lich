// The sessions the one on screen can hand work to, grouped by the project they
// belong to. This one addresses a session by the label on its card, and every
// provider can be a target because the relay types at the target's prompt
// rather than using any messaging channel of its own (docs/cli.md).
//
// The list spans every open project, because the sidebar shows one project at a
// time and walking to another would change which session is sending.
//
// A card whose terminal was never opened is listed like any other. It has no
// PTY, so the relay refuses it — and says so, in the session that asked. Hiding
// it here would mean tracking spawn state outside the component that owns it,
// for a case the sender is told about anyway.

import type { Project } from "@/lib/api-types"
import type { SessionKind, SessionState } from "./sessions"
import { sessionsOf } from "./sessions"

export interface DelegateTarget {
  id: string
  /** The label on the card, which is also how the relay addresses it. */
  label: string
  kind: SessionKind
}

export interface DelegateGroup {
  projectId: string
  projectName: string
  targets: DelegateTarget[]
}

/**
 * Every session across every open project except the one doing the sending, in
 * project order. A project with nothing to offer is left out, so an empty
 * result means there is nobody to hand work to.
 */
export function delegateTargets(
  projects: readonly Project[],
  sessions: SessionState,
  sourceId: string,
): DelegateGroup[] {
  const groups: DelegateGroup[] = []
  for (const project of projects) {
    const targets = sessionsOf(sessions, project.id)
      .filter((session) => session.id !== sourceId)
      .map((session) => ({ id: session.id, label: session.label, kind: session.kind }))
    if (targets.length > 0) {
      groups.push({ projectId: project.id, projectName: project.name, targets })
    }
  }
  return groups
}
