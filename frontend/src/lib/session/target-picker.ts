// The search behind SessionTargetPicker: flattens delegate targets (grouped by
// project, see delegate-targets.ts) into rows, filters them by query and groups
// what survives back under its project — kept pure so the matching and the
// index the keyboard walks are testable without a render.

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

export interface TargetGroup {
  projectId: string
  projectName: string
  // Each row keeps its position in the flat result list, which is what the
  // arrow keys move through: the grouping is for the eye only, and a row that
  // renders under a heading still has to answer to one index.
  rows: { row: TargetRow; index: number }[]
}

export function groupTargetRows(rows: readonly TargetRow[]): TargetGroup[] {
  // Grouped in first-appearance order, which — since the flatten walks the
  // groups the caller passed in — is the order those projects arrived in.
  const byProject = new Map<string, TargetGroup>()
  rows.forEach((row, index) => {
    const group = byProject.get(row.projectId) ?? {
      projectId: row.projectId,
      projectName: row.projectName,
      rows: [],
    }
    group.rows.push({ row, index })
    byProject.set(row.projectId, group)
  })
  return [...byProject.values()]
}
