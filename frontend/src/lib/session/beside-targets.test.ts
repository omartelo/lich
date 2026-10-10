import { describe, expect, it } from "vitest"
import { besideTargets } from "./beside-targets"
import type { SessionState } from "./sessions"

const PROJECTS = [
  { id: "p1", name: "lich", path: "/home/me/code/lich" },
  { id: "p2", name: "lich-plugin", path: "/home/me/code/lich-plugin" },
]

const state: SessionState = {
  p1: {
    activeId: "a",
    nextSeq: 3,
    sessions: [
      { id: "a", label: "orchestrator", kind: "claude" },
      { id: "b", label: "fix-relay", kind: "codex" },
    ],
  },
  p2: {
    activeId: "c",
    nextSeq: 2,
    sessions: [{ id: "c", label: "hook-payload", kind: "claude" }],
  },
}

const ids = (groups: ReturnType<typeof besideTargets>) =>
  groups.map((group) => [group.projectId, group.targets.map((target) => target.id)])

describe("besideTargets", () => {
  it("lists the routed project first, then every other open project", () => {
    expect(ids(besideTargets(PROJECTS, state, "p2", ["c"]))).toEqual([["p1", ["a", "b"]]])
    expect(ids(besideTargets(PROJECTS, state, "p2", []))).toEqual([
      ["p2", ["c"]],
      ["p1", ["a", "b"]],
    ])
  })

  it("leaves out what is on screen, and a project left with nothing", () => {
    expect(ids(besideTargets(PROJECTS, state, "p1", ["a", "c"]))).toEqual([["p1", ["b"]]])
  })
})
