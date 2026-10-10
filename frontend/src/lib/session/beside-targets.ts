// The sessions a pane's + offers: every session of every open project that is
// not on the wall on screen already, grouped by project with the routed one
// first — the session most often wanted beside is a sibling, and the reach into
// other projects is what the list adds. Built on the delegate picker's groups so
// the two lists agree on what a project's sessions are and in what order.

import type { Project } from "@/lib/api-types"
import { type DelegateGroup, delegateTargets } from "./delegate-targets"
import type { SessionState } from "./sessions"

export function besideTargets(
  projects: readonly Project[],
  sessions: SessionState,
  projectId: string,
  onScreen: readonly string[],
): DelegateGroup[] {
  const shown = new Set(onScreen)
  const groups = delegateTargets(projects, sessions, "").flatMap((group) => {
    const targets = group.targets.filter((target) => !shown.has(target.id))
    return targets.length > 0 ? [{ ...group, targets }] : []
  })
  return [
    ...groups.filter((group) => group.projectId === projectId),
    ...groups.filter((group) => group.projectId !== projectId),
  ]
}
