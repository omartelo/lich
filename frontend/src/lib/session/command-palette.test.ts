import { describe, expect, it } from "vitest"
import {
  filterPalette,
  historyAction,
  historyRows,
  matchesQuery,
  nextTab,
  type PaletteHistory,
  paletteGroups,
  paletteMessages,
  paletteSessions,
  paletteTabCount,
  rankSessions,
  rowKey,
} from "./command-palette"
import type { ClosedSession, Project } from "@/lib/api-types"
import type { SessionState } from "./sessions"

const projects: Project[] = [
  { id: "p1", name: "lich", path: "/home/u/try/skipo" },
  { id: "p2", name: "revu", path: "/home/u/try/revu" },
]

const closed: Project[] = [
  { id: "c1", name: "lich-plugin", path: "/home/u/try/lich-plugin" },
  { id: "c2", name: "controle-de-licitacao", path: "/home/u/snk/controle-de-licitacao" },
  { id: "c3", name: "gh-turnkey", path: "/home/u/snk/gh-turnkey" },
  { id: "c4", name: "policy-engine", path: "/home/u/hycastle/policy-engine" },
]

const sessions: SessionState = {
  p1: {
    sessions: [
      { id: "s1", label: "Fix flaky test", kind: "claude" },
      { id: "s2", label: "build watch", kind: "shell", path: "/home/u/try/skipo/frontend" },
    ],
    activeId: "s1",
    nextSeq: 3,
  },
  p2: {
    sessions: [{ id: "s3", label: "Wire revu auth middleware", kind: "claude" }],
    activeId: "s3",
    nextSeq: 2,
  },
  // A project with sessions but no open tab — must be dropped.
  ghost: {
    sessions: [{ id: "s9", label: "orphan", kind: "claude" }],
    activeId: "s9",
    nextSeq: 2,
  },
}

describe("paletteSessions", () => {
  it("flattens sessions of open projects with their project", () => {
    const rows = paletteSessions(projects, sessions)
    expect(rows.map((r) => r.sessionId)).toEqual(["s1", "s2", "s3"])
    expect(rows[0]).toMatchObject({ projectId: "p1", projectName: "lich", label: "Fix flaky test" })
  })

  it("uses the worktree path when set, else the project path", () => {
    const rows = paletteSessions(projects, sessions)
    expect(rows.find((r) => r.sessionId === "s1")?.path).toBe("/home/u/try/skipo")
    expect(rows.find((r) => r.sessionId === "s2")?.path).toBe("/home/u/try/skipo/frontend")
  })

  it("drops sessions whose project has no open tab", () => {
    const rows = paletteSessions(projects, sessions)
    expect(rows.some((r) => r.projectId === "ghost")).toBe(false)
  })
})

describe("matchesQuery", () => {
  it("matches when every token is present, in any order", () => {
    expect(matchesQuery("Wire revu auth middleware", "revu auth")).toBe(true)
    expect(matchesQuery("Wire revu auth middleware", "auth revu")).toBe(true)
  })

  it("is case-insensitive and matches an empty query", () => {
    expect(matchesQuery("lich", "LICH")).toBe(true)
    expect(matchesQuery("anything", "  ")).toBe(true)
  })

  it("fails when a token is absent", () => {
    expect(matchesQuery("Wire revu auth", "revu payments")).toBe(false)
  })
})

describe("filterPalette", () => {
  const all = paletteSessions(projects, sessions)

  it("filters sessions by label, project name and path", () => {
    expect(filterPalette("flaky", all, projects).sessions.map((s) => s.sessionId)).toEqual(["s1"])
    expect(filterPalette("revu", all, projects).sessions.map((s) => s.sessionId)).toEqual(["s3"])
    expect(filterPalette("frontend", all, projects).sessions.map((s) => s.sessionId)).toEqual([
      "s2",
    ])
  })

  it("filters projects by name and path", () => {
    expect(filterPalette("revu", all, projects).projects.map((p) => p.id)).toEqual(["p2"])
    expect(filterPalette("skipo", all, projects).projects.map((p) => p.id)).toEqual(["p1"])
  })

  it("returns everything for an empty query", () => {
    const r = filterPalette("", all, projects)
    expect(r.sessions).toHaveLength(3)
    expect(r.projects).toHaveLength(2)
  })

  it("filters closed projects by name and path", () => {
    expect(filterPalette("plugin", all, projects, closed).closed.map((p) => p.id)).toEqual(["c1"])
    expect(filterPalette("snk", all, projects, closed).closed.map((p) => p.id)).toEqual([
      "c2",
      "c3",
    ])
  })

  it("has no closed projects when none were passed", () => {
    expect(filterPalette("plugin", all, projects).closed).toEqual([])
  })
})

describe("paletteGroups", () => {
  const all = paletteSessions(projects, sessions)
  const results = filterPalette("", all, projects, closed)
  const messages = paletteMessages([{ id: "s1", snippet: "flaky again", count: 1 }], all)

  it("shows the kinds worth interrupting for, and leaves the closed projects to their tab", () => {
    const groups = paletteGroups("All", results, messages)
    expect(groups.map((g) => g.label)).toEqual(["Sessions", "Projects", "Messages"])
    // Nothing was cut here, so the header has nothing to report.
    const projectGroup = groups.find((g) => g.label === "Projects")
    expect(projectGroup?.rows).toHaveLength(2)
    expect(projectGroup?.total).toBe(2)
  })

  it("cuts a group to five rows and says what it left out", () => {
    const many = Array.from({ length: 7 }, (_, i) => ({ ...all[0], sessionId: `x${i}` }))
    const groups = paletteGroups("All", { ...results, sessions: many }, [])
    const sessionGroup = groups.find((g) => g.label === "Sessions")
    expect(sessionGroup?.rows).toHaveLength(5)
    expect(sessionGroup?.total).toBe(7)
  })

  it("lists one kind whole under its own tab", () => {
    const groups = paletteGroups("Projects", results, messages)
    expect(groups.map((g) => g.label)).toEqual(["Open", "Closed"])
    expect(groups[1]?.rows).toHaveLength(4)
    expect(paletteGroups("Sessions", results, messages)[0]?.rows).toHaveLength(3)
  })

  it("drops a group with no rows", () => {
    const empty = filterPalette("nothing-matches-this", all, projects, closed)
    expect(paletteGroups("All", empty, [])).toEqual([])
    expect(paletteGroups("Projects", empty, [])).toEqual([])
  })

  it("carries what a row needs to be run", () => {
    const rows = paletteGroups("All", results, messages).flatMap((g) => g.rows)
    expect(rows[0]).toEqual({ kind: "session", session: all[0] })
    const reopen = paletteGroups("Projects", results, messages).flatMap((g) => g.rows)
    expect(reopen.find((r) => r.kind === "closed")).toEqual({ kind: "closed", project: closed[0] })
  })
})

describe("rankSessions", () => {
  const all = paletteSessions(projects, sessions)

  it("puts the sessions holding a turn first and the shells last", () => {
    const ranked = rankSessions(all, new Set(["s3"]))
    expect(ranked.map((r) => r.sessionId)).toEqual(["s3", "s1", "s2"])
  })

  it("keeps the tab order between sessions of equal rank", () => {
    expect(rankSessions(all, new Set()).map((r) => r.sessionId)).toEqual(["s1", "s3", "s2"])
  })

  it("ranks a running shell with the running sessions", () => {
    expect(rankSessions(all, new Set(["s2"])).map((r) => r.sessionId)).toEqual(["s2", "s1", "s3"])
  })

  it("leaves the list it was given alone", () => {
    const before = all.map((r) => r.sessionId)
    rankSessions(all, new Set(["s3"]))
    expect(all.map((r) => r.sessionId)).toEqual(before)
  })
})

describe("rowKey", () => {
  // The session and the message about it share an id on purpose: they key the
  // same string and never land in the same group.
  it("keys every row by what it opens", () => {
    const all = paletteSessions(projects, sessions)
    const results = filterPalette("", all, projects, closed)
    const messages = paletteMessages([{ id: "s1", snippet: "x", count: 1 }], all)
    const rows = paletteGroups("All", results, messages).flatMap((g) => g.rows)
    expect(rows.map(rowKey)).toEqual(["s1", "s2", "s3", "p1", "p2", "s1"])
  })
})

describe("paletteTabCount", () => {
  const all = paletteSessions(projects, sessions)
  const results = filterPalette("", all, projects, closed)

  it("counts open and closed projects together, and nothing for All", () => {
    expect(paletteTabCount("Projects", results, [])).toBe(6)
    expect(paletteTabCount("Sessions", results, [])).toBe(3)
    expect(paletteTabCount("Messages", results, [])).toBe(0)
    expect(paletteTabCount("All", results, [])).toBeNull()
  })
})

describe("nextTab", () => {
  it("wraps at both ends", () => {
    expect(nextTab("All", 1)).toBe("Sessions")
    expect(nextTab("Messages", 1)).toBe("History")
    expect(nextTab("History", 1)).toBe("All")
    expect(nextTab("All", -1)).toBe("History")
  })
})

describe("paletteMessages", () => {
  const all = paletteSessions(projects, sessions)

  it("carries the session a match belongs to, so the row can open it", () => {
    const rows = paletteMessages([{ id: "s3", snippet: "wire the auth middleware", count: 2 }], all)
    expect(rows).toEqual([
      {
        sessionId: "s3",
        projectId: "p2",
        projectName: "revu",
        label: "Wire revu auth middleware",
        kind: "claude",
        path: "/home/u/try/revu",
        snippet: "wire the auth middleware",
        count: 2,
      },
    ])
  })

  it("keeps the order the search returned", () => {
    const rows = paletteMessages(
      [
        { id: "s3", snippet: "third", count: 1 },
        { id: "s1", snippet: "first", count: 1 },
      ],
      all,
    )
    expect(rows.map((r) => r.sessionId)).toEqual(["s3", "s1"])
  })

  // The search is debounced, so its reply can name a session closed while it
  // was in flight. That row would have nowhere to jump to.
  it("drops a match whose session is gone", () => {
    const rows = paletteMessages(
      [
        { id: "s9", snippet: "orphan", count: 1 },
        { id: "s1", snippet: "still here", count: 1 },
      ],
      all,
    )
    expect(rows.map((r) => r.sessionId)).toEqual(["s1"])
  })

  it("is empty when the search found nothing or never ran", () => {
    expect(paletteMessages([], all)).toEqual([])
    expect(paletteMessages(null, all)).toEqual([])
  })
})

const parked: ClosedSession[] = [
  {
    id: "h1",
    projectId: "p1",
    projectName: "lich",
    projectPath: "/home/u/try/skipo",
    label: "Wire the relay inbox",
    kind: "claude",
    path: "/home/u/wt/lich/relay-inbox",
    closedAt: 1_700_000_200,
  },
  {
    id: "h2",
    projectId: "p1",
    projectName: "lich",
    projectPath: "/home/u/try/skipo",
    label: "Conpty handle recycling",
    kind: "claude",
    path: "/home/u/wt/lich/conpty",
    closedAt: 1_700_000_100,
  },
  {
    id: "h3",
    projectId: "p2",
    projectName: "revu",
    projectPath: "/home/u/try/revu",
    label: "vitest --ui",
    kind: "shell",
    path: "/home/u/wt/revu/gone",
    closedAt: 1_700_000_000,
  },
]

const parkedBranches = {
  "/home/u/wt/lich/relay-inbox": "feat/relay-inbox",
  "/home/u/wt/lich/conpty": "fix/conpty-handle",
}

describe("historyRows", () => {
  it("hangs the live branch off each parked row, and marks the checkouts that are gone", () => {
    const rows = historyRows(parked, parkedBranches, new Set(["/home/u/wt/revu/gone"]))
    expect(rows.map((r) => [r.id, r.branch, r.gone])).toEqual([
      ["h1", "feat/relay-inbox", false],
      ["h2", "fix/conpty-handle", false],
      ["h3", "", true],
    ])
  })

  it("leaves a checkout that is present but nameless unmarked — no branch is not gone", () => {
    const rows = historyRows(parked.slice(0, 1), {}, new Set())
    expect(rows[0]?.branch).toBe("")
    expect(rows[0]?.gone).toBe(false)
  })

  it("survives a store that answered before git did", () => {
    expect(historyRows([], {}, new Set())).toEqual([])
    expect(historyRows(parked, {}, new Set())).toHaveLength(3)
  })
})

describe("history search", () => {
  const rows = historyRows(parked, parkedBranches, new Set())

  it("narrows on the branch, which the path cannot stand in for", () => {
    const hit = filterPalette("conpty-handle", [], [], [], rows).history
    expect(hit.map((h) => h.id)).toEqual(["h2"])
  })

  it("narrows on the label, the project and the path too", () => {
    expect(filterPalette("relay", [], [], [], rows).history.map((h) => h.id)).toEqual(["h1"])
    expect(filterPalette("revu", [], [], [], rows).history.map((h) => h.id)).toEqual(["h3"])
    expect(filterPalette("wt/lich", [], [], [], rows).history.map((h) => h.id)).toEqual([
      "h1",
      "h2",
    ])
  })

  it("takes every token, so two words narrow further than one", () => {
    expect(filterPalette("lich relay", [], [], [], rows).history.map((h) => h.id)).toEqual(["h1"])
    expect(filterPalette("lich nothing", [], [], [], rows).history).toEqual([])
  })

  it("is empty rather than everything when no history was loaded", () => {
    expect(filterPalette("relay", [], [], []).history).toEqual([])
  })
})

describe("the History tab", () => {
  const rows = historyRows(parked, parkedBranches, new Set())
  const results = filterPalette("", [], projects, closed, rows)

  it("lists the parked sessions whole, under their own heading", () => {
    const groups = paletteGroups("History", results, [])
    expect(groups).toHaveLength(1)
    expect(groups[0]?.label).toBe("Closed sessions")
    expect(groups[0]?.rows.map((r) => rowKey(r))).toEqual(["h1", "h2", "h3"])
  })

  it("keeps history out of All, which is the mid-work posture", () => {
    const kinds = paletteGroups("All", results, []).flatMap((g) => g.rows.map((r) => r.kind))
    expect(kinds).not.toContain("history")
  })

  it("counts what it holds", () => {
    expect(paletteTabCount("History", results, [])).toBe(3)
  })

  it("draws no group at all when nothing has ever been closed", () => {
    const none = filterPalette("", [], projects, closed, [])
    expect(paletteGroups("History", none, [])).toEqual([])
  })
})

describe("historyAction", () => {
  const rows = historyRows(parked, parkedBranches, new Set(["/home/u/wt/revu/gone"]))

  it("resumes a row whose checkout is still there and forgets one whose is not", () => {
    expect(historyAction(rows[0] as PaletteHistory)).toBe("resume")
    expect(historyAction(rows[2] as PaletteHistory)).toBe("forget")
  })
})
