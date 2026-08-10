import { describe, expect, it } from "vitest"
import { mentionTargets } from "./mention-targets"
import type { SessionState } from "./sessions"

const PROJECTS = [
  { id: "p1", name: "lich", path: "/home/me/code/lich" },
  { id: "p2", name: "lich-plugin", path: "/home/me/code/lich-plugin" },
]

const state: SessionState = {
  p1: {
    activeId: "aaaa1111",
    nextSeq: 4,
    sessions: [
      { id: "aaaa1111", label: "Sender", kind: "claude" },
      {
        id: "bbbb2222",
        label: "Fix migration drift",
        kind: "claude",
        path: "/home/me/wt/migration",
      },
      { id: "cccc3333", label: "A shell", kind: "shell" },
    ],
  },
  p2: {
    activeId: "",
    nextSeq: 2,
    sessions: [{ id: "dddd4444", label: "Hook fixtures", kind: "claude" }],
  },
}

describe("mentionTargets", () => {
  it("spans every open project, in project order", () => {
    const groups = mentionTargets(PROJECTS, state, "aaaa1111")
    expect(groups.map((g) => g.projectName)).toEqual(["lich", "lich-plugin"])
  })

  it("leaves out the session doing the sending", () => {
    const ids = mentionTargets(PROJECTS, state, "aaaa1111").flatMap((g) =>
      g.targets.map((t) => t.id),
    )
    expect(ids).not.toContain("aaaa1111")
    expect(ids).toContain("bbbb2222")
  })

  it("offers only Claude sessions", () => {
    const ids = mentionTargets(PROJECTS, state, "aaaa1111").flatMap((g) =>
      g.targets.map((t) => t.id),
    )
    expect(ids).not.toContain("cccc3333")
  })

  it("names a worktree session after its checkout, others after the project", () => {
    const [lich, plugin] = mentionTargets(PROJECTS, state, "aaaa1111")
    expect(lich.targets[0]).toMatchObject({ label: "Fix migration drift", name: "migration-bbbb" })
    expect(plugin.targets[0]).toMatchObject({ name: "lich-plugin-dddd" })
  })

  it("drops a project with nothing to point at", () => {
    const groups = mentionTargets(PROJECTS, state, "dddd4444")
    expect(groups.map((g) => g.projectId)).toEqual(["p1"])
  })

  it("is empty when the sender is the only Claude session", () => {
    const alone: SessionState = {
      p1: { activeId: "aaaa1111", nextSeq: 2, sessions: [state.p1.sessions[0]] },
    }
    expect(mentionTargets(PROJECTS, alone, "aaaa1111")).toEqual([])
  })
})
