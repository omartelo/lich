import { describe, expect, it } from "vitest"
import { filterTargetRows, flattenTargetGroups } from "./target-picker"
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
