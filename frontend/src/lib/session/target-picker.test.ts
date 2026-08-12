import { describe, expect, it } from "vitest"
import { filterTargetRows, flattenTargetGroups, groupTargetRows } from "./target-picker"
import type { DelegateGroup } from "./delegate-targets"

const GROUPS: DelegateGroup[] = [
  {
    projectId: "p1",
    projectName: "lich",
    targets: [
      { id: "a", label: "Fix migration drift", kind: "claude" },
      { id: "b", label: "Wire revu auth", kind: "codex" },
    ],
  },
  {
    projectId: "p2",
    projectName: "lich-plugin",
    targets: [{ id: "c", label: "Hook fixtures", kind: "claude" }],
  },
]

describe("flattenTargetGroups", () => {
  it("flattens every group's targets, carrying the project along", () => {
    const rows = flattenTargetGroups(GROUPS)
    expect(rows.map((r) => r.target.id)).toEqual(["a", "b", "c"])
    expect(rows[0]).toMatchObject({ projectId: "p1", projectName: "lich" })
    expect(rows[2]).toMatchObject({ projectId: "p2", projectName: "lich-plugin" })
  })

  it("is empty for no groups", () => {
    expect(flattenTargetGroups([])).toEqual([])
  })
})

describe("filterTargetRows", () => {
  const rows = flattenTargetGroups(GROUPS)

  it("matches on the target's label", () => {
    const result = filterTargetRows("migration", rows)
    expect(result.map((r) => r.target.id)).toEqual(["a"])
  })

  it("matches on the project name", () => {
    const result = filterTargetRows("plugin", rows)
    expect(result.map((r) => r.target.id)).toEqual(["c"])
  })

  it("is every row for an empty query", () => {
    expect(filterTargetRows("", rows)).toHaveLength(3)
  })

  it("is a light fuzzy match: every token has to appear, in any field", () => {
    const result = filterTargetRows("revu auth", rows)
    expect(result.map((r) => r.target.id)).toEqual(["b"])
  })

  it("matches nothing for a query none of the fields contain", () => {
    expect(filterTargetRows("nonexistent", rows)).toEqual([])
  })
})

describe("groupTargetRows", () => {
  const rows = flattenTargetGroups(GROUPS)

  it("groups by project, in the order the projects first appear", () => {
    const grouped = groupTargetRows(rows)
    expect(grouped.map((g) => g.projectId)).toEqual(["p1", "p2"])
    expect(grouped[0].rows.map((r) => r.row.target.id)).toEqual(["a", "b"])
    expect(grouped[1].rows.map((r) => r.row.target.id)).toEqual(["c"])
  })

  it("keeps each row's position in the flat list, which is what the keyboard walks", () => {
    const grouped = groupTargetRows(rows)
    expect(grouped.flatMap((g) => g.rows.map((r) => r.index))).toEqual([0, 1, 2])
  })

  // Filtering can empty a project out entirely; the survivors must still carry
  // indices into the filtered list, not into the list before it.
  it("indexes the filtered rows, not the ones they came from", () => {
    const grouped = groupTargetRows(filterTargetRows("plugin", rows))
    expect(grouped.map((g) => g.projectId)).toEqual(["p2"])
    expect(grouped[0].rows).toEqual([{ row: expect.anything(), index: 0 }])
  })

  it("is empty for no rows", () => {
    expect(groupTargetRows([])).toEqual([])
  })
})
