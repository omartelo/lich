import { describe, expect, it } from "vitest"
import {
  canRace,
  consolidationPrompt,
  consolidatorOf,
  folderCheckouts,
  planRace,
  raceRivals,
  raceTask,
  rememberConsolidator,
  rememberRaceTask,
  toggleAgent,
} from "./agent-race"
import type { Session } from "./sessions"

describe("planRace", () => {
  it("names one branch per agent after the shared name, filed under that name", () => {
    expect(planRace("fix-auth", "anything", ["claude", "codex"])).toEqual({
      folder: "fix-auth",
      checkouts: [
        { kind: "claude", branch: "fix-auth-claude" },
        { kind: "codex", branch: "fix-auth-codex" },
      ],
    })
  })

  it("names the race after the task when no name was typed", () => {
    const plan = planRace("", "Fix the auth redirect", ["opencode"])
    expect(plan.folder).toBe("fix-the-auth-redirect")
    expect(plan.checkouts).toEqual([{ kind: "opencode", branch: "fix-the-auth-redirect-opencode" }])
  })
})

describe("toggleAgent", () => {
  const offered = ["claude", "codex", "opencode"] as const

  it("adds an agent in the order the providers are offered", () => {
    expect(toggleAgent([...offered], ["opencode"], "claude")).toEqual(["claude", "opencode"])
  })

  it("removes a ticked agent", () => {
    expect(toggleAgent([...offered], ["claude", "codex"], "claude")).toEqual(["codex"])
  })

  it("unticks the last agent too, which leaves no race to start", () => {
    expect(toggleAgent([...offered], ["codex"], "codex")).toEqual([])
  })
})

const card = (id: string, extra: Partial<Session> = {}): Session => ({
  id,
  label: id,
  kind: "claude",
  ...extra,
})

describe("raceRivals", () => {
  const winner = card("w", { path: "/wt/a-claude", folder: "a" })

  it("returns every other worktree filed in the winner's folder", () => {
    const codex = card("c", { path: "/wt/a-codex", folder: "a", kind: "codex" })
    const crush = card("r", { path: "/wt/a-crush", folder: "a", kind: "crush" })
    expect(raceRivals([winner, codex, crush], winner)).toEqual([
      { path: "/wt/a-codex", sessions: [codex] },
      { path: "/wt/a-crush", sessions: [crush] },
    ])
  })

  it("groups the sessions sharing one checkout under that checkout", () => {
    const agent = card("c", { path: "/wt/a-codex", folder: "a" })
    const shell = card("t", { path: "/wt/a-codex", folder: "a", kind: "shell" })
    expect(raceRivals([winner, agent, shell], winner)).toEqual([
      { path: "/wt/a-codex", sessions: [agent, shell] },
    ])
  })

  it("keeps the winner's checkout, the project directory and other folders", () => {
    const beside = card("b", { path: "/wt/a-claude", folder: "a", kind: "shell" })
    const project = card("p", { folder: "a" })
    const elsewhere = card("e", { path: "/wt/b-codex", folder: "b" })
    const unfiled = card("u", { path: "/wt/c" })
    expect(raceRivals([winner, beside, project, elsewhere, unfiled], winner)).toEqual([])
  })

  it("keeps a checkout that holds a pinned session", () => {
    const pinned = card("c", { path: "/wt/a-codex", folder: "a", pinned: true })
    const beside = card("t", { path: "/wt/a-codex", folder: "a", kind: "shell" })
    expect(raceRivals([winner, pinned, beside], winner)).toEqual([])
  })

  it("has no rivals for a session in no folder", () => {
    const lone = card("l", { path: "/wt/l" })
    expect(raceRivals([lone, card("o", { path: "/wt/o" })], lone)).toEqual([])
  })
})

describe("folderCheckouts", () => {
  it("returns every worktree in the folder when nothing is kept", () => {
    const a = card("a", { path: "/wt/a", folder: "f" })
    const b = card("b", { path: "/wt/b", folder: "f" })
    const project = card("p", { folder: "f" })
    expect(folderCheckouts([a, b, project], "f")).toEqual([
      { path: "/wt/a", sessions: [a] },
      { path: "/wt/b", sessions: [b] },
    ])
  })
})

describe("canRace", () => {
  // Measured (docs/ceilings.md): none of the four reports session-start from
  // its own prompt before its first turn.
  it("leaves out the providers with no signal from their own prompt", () => {
    expect(canRace("codex")).toBe(false)
    expect(canRace("antigravity")).toBe(false)
    expect(canRace("crush")).toBe(false)
    expect(canRace("opencode")).toBe(false)
    for (const kind of ["claude", "omp", "cursor", "kiro"] as const) {
      expect(canRace(kind)).toBe(true)
    }
  })
})

describe("consolidationPrompt", () => {
  const sources = [
    { branch: "fix-claude", path: "/wt/fix-claude" },
    { branch: "fix-codex", path: "/wt/fix-codex" },
  ]

  it("lists every branch with its worktree, the task, and the diffs to read", () => {
    const prompt = consolidationPrompt("main", sources, "fix the redirect")
    expect(prompt).toContain("2 agents worked the same task in parallel")
    expect(prompt).toContain("- fix-claude, at /wt/fix-claude")
    expect(prompt).toContain("- fix-codex, at /wt/fix-codex")
    expect(prompt).toContain("fix the redirect")
    expect(prompt).toContain("git diff main...<branch>")
    expect(prompt).toContain("git -C <path> diff")
    expect(prompt).toContain("what you took from which branch")
  })

  it("leaves a place to write the task when the window forgot it", () => {
    expect(consolidationPrompt("main", sources, "")).toContain(
      "The task each one was given: (write it here)",
    )
  })
})

describe("race memory", () => {
  it("keeps the task and the consolidation per project and folder", () => {
    rememberRaceTask("p1", "fix", "the task")
    rememberConsolidator("p1", "fix", "s9")
    expect(raceTask("p1", "fix")).toBe("the task")
    expect(consolidatorOf("p1", "fix")).toBe("s9")
    expect(raceTask("p2", "fix")).toBe("")
    expect(consolidatorOf("p1", "other")).toBe("")
  })
})
