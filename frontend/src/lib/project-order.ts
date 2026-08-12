import type { Project } from "@/lib/api-types"

// neighborProjectId returns the project one step from `activeId` in the order the
// tab strip draws — Home pinned first, the rest in their stored order — wrapping
// at both ends. "" means there is nowhere to go (a single tab, or none), and the
// caller leaves the screen alone.
export function neighborProjectId(
  projects: readonly Project[],
  homeId: string | null,
  activeId: string | undefined,
  step: 1 | -1,
): string {
  const home = projects.filter((project) => project.id === homeId)
  const ids = [...home, ...projects.filter((project) => project.id !== homeId)].map((p) => p.id)
  if (ids.length < 2) {
    return ""
  }
  const index = activeId ? ids.indexOf(activeId) : -1
  // Nowhere to step from (the landing screen, or a tab just closed): the press
  // still moves, onto the end the step comes from.
  if (index === -1) {
    return step === 1 ? ids[0] : ids[ids.length - 1]
  }
  return ids[(index + step + ids.length) % ids.length]
}
