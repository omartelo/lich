import type { BaseStatus } from "@/lib/api-types"

// How many conflicting paths the tooltip names before it starts counting. A
// merge can collide on dozens of files, and a tooltip that grows with them
// stops being a tooltip — three names say which corner of the tree is fighting,
// which is all the readout is for.
const NAMED_CONFLICTS = 3

/** What the sidebar and the tooltip draw for one checkout's base standing. */
export interface BaseReadout {
  /** Which mark the card shows: a plain behind count, or the conflict alert. */
  kind: "behind" | "conflict"
  /** The number beside the mark — commits behind, or files in conflict. */
  count: number
  /** "3 commits behind origin/main"; null when the base has not moved. */
  behind: string | null
  /** "Conflicts in 8 files"; null when the merge is clean. */
  conflict: string | null
  /** The named conflicting paths, at most NAMED_CONFLICTS of them. */
  paths: string[]
  /** How many conflicting paths were left unnamed. */
  more: number
}

// baseReadout turns the service's answer into everything the card renders, so
// the component stays a template: the frontend gate runs in node and never
// renders, which makes anything decided inside the JSX untestable.
//
// It yields null for the case that is true nearly all day — a base that has not
// moved and nothing to collide with. A mark that is always on screen is not a
// mark, and the absent readout is what keeps the card the height it is today.
export function baseReadout(status: BaseStatus | null): BaseReadout | null {
  if (!status) {
    return null
  }
  const conflicts = status.conflicts ?? []
  if (status.behind === 0 && conflicts.length === 0) {
    return null
  }
  return {
    kind: conflicts.length > 0 ? "conflict" : "behind",
    count: conflicts.length > 0 ? conflicts.length : status.behind,
    behind: status.behind > 0 ? `${count(status.behind, "commit")} behind ${status.base}` : null,
    conflict: conflicts.length > 0 ? `Conflicts in ${count(conflicts.length, "file")}` : null,
    paths: conflicts.slice(0, NAMED_CONFLICTS),
    more: Math.max(0, conflicts.length - NAMED_CONFLICTS),
  }
}

function count(n: number, noun: string): string {
  return `${n} ${noun}${n === 1 ? "" : "s"}`
}
