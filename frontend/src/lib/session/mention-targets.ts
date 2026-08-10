// The sessions one Claude session can be pointed at, grouped by the project
// they belong to. lich never carries the message itself: it writes the target's
// roster name at the sender's own prompt, and the Claude there addresses it with
// the SendMessage tool it already has (docs: cross-session messaging).
//
// The list spans every open project, because the sidebar shows one project at a
// time and walking to another would change which session is sending.
//
// A card whose terminal was never opened is listed like any other — it has no
// process, so no inbox socket, and the sender's Claude reports the name as
// unreachable. Filtering those out would mean tracking spawn state outside the
// component that owns it, for a case the user is told about anyway.

import type { Project } from "@/lib/api-types"
import { peerName } from "./peer-name"
import type { SessionState } from "./sessions"
import { sessionsOf } from "./sessions"

export interface MentionTarget {
  id: string
  label: string
  /** The name it answers to in the roster — what lands at the prompt. */
  name: string
}

export interface MentionGroup {
  projectId: string
  projectName: string
  targets: MentionTarget[]
}

/**
 * Claude sessions across every open project, minus the one doing the sending,
 * grouped in project order. A project with nothing to offer is left out, so an
 * empty result means there is nobody to point at.
 */
export function mentionTargets(
  projects: readonly Project[],
  sessions: SessionState,
  sourceId: string,
): MentionGroup[] {
  const groups: MentionGroup[] = []
  for (const project of projects) {
    const targets = sessionsOf(sessions, project.id)
      .filter((session) => session.kind === "claude" && session.id !== sourceId)
      .map((session) => ({
        id: session.id,
        label: session.label,
        // The path the session spawned in, never its live cwd: the name was
        // fixed at spawn, and a `cd` in the terminal does not rename it.
        name: peerName(session.path || project.path, session.id),
      }))
    if (targets.length > 0) {
      groups.push({ projectId: project.id, projectName: project.name, targets })
    }
  }
  return groups
}
