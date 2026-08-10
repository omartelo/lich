import { describe, expect, it } from "vitest"
import { delegateTargets } from "./delegate-targets"
import type { SessionState } from "./sessions"

const PROJECTS = [
  { id: "p1", name: "lich", path: "/home/me/code/lich" },
  { id: "p2", name: "revu", path: "/home/me/code/revu" },
]

const state: SessionState = {
  p1: {
    activeId: "aaaa1111",
    nextSeq: 4,
    sessions: [
      { id: "aaaa1111", label: "auth", kind: "claude" },
      { id: "bbbb2222", label: "docs", kind: "codex" },
      { id: "cccc3333", label: "api", kind: "crush" },
    ],
  },
  p2: {
    activeId: "",
    nextSeq: 2,
    sessions: [{ id: "dddd4444", label: "web", kind: "opencode" }],
  },
}

describe("delegateTargets", () => {
  it("offers every other session, whatever provider it runs", () => {
    const groups = delegateTargets(PROJECTS, state, "aaaa1111")

    expect(groups.map((group) => group.projectName)).toEqual(["lich", "revu"])
    expect(groups[0].targets.map((target) => target.label)).toEqual(["docs", "api"])
    expect(groups[1].targets.map((target) => target.label)).toEqual(["web"])
  })

  it("never offers the session doing the sending", () => {
    const labels = delegateTargets(PROJECTS, state, "bbbb2222")
      .flatMap((group) => group.targets)
      .map((target) => target.label)

    expect(labels).not.toContain("docs")
    expect(labels).toEqual(["auth", "api", "web"])
  })

  it("drops a project with nothing left to offer", () => {
    const alone: SessionState = {
      p1: { activeId: "", nextSeq: 1, sessions: [] },
      p2: state.p2,
    }
    expect(delegateTargets(PROJECTS, alone, "dddd4444")).toEqual([])
  })

  it("is empty when this is the only session open", () => {
    const alone: SessionState = {
      p1: { activeId: "aaaa1111", nextSeq: 2, sessions: [state.p1.sessions[0]] },
    }
    expect(delegateTargets(PROJECTS, alone, "aaaa1111")).toEqual([])
  })
})
