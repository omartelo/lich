// The search behind SessionTargetPicker: flattens delegate targets (grouped by
// project, see delegate-targets.ts) into rows and filters them by query, kept
// pure so the matching is testable without a render.

import { matchesQuery } from "./command-palette"
import type { DelegateGroup, DelegateTarget } from "./delegate-targets"

export interface TargetRow {
  target: DelegateTarget
  projectId: string
  projectName: string
}

export function flattenTargetGroups(groups: readonly DelegateGroup[]): TargetRow[] {
  const rows: TargetRow[] = []
  for (const group of groups) {
    for (const target of group.targets) {
      rows.push({ target, projectId: group.projectId, projectName: group.projectName })
    }
  }
  return rows
}

export function filterTargetRows(query: string, rows: readonly TargetRow[]): TargetRow[] {
  return rows.filter((row) => matchesQuery(`${row.target.label} ${row.projectName}`, query))
}
