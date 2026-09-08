import { describe, expect, it } from "vitest"
import {
  filterPalette,
  historyAction,
  historyIndexNote,
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

  it("says how many closed projects the store matched past the page it sent", () => {
    const cut = filterPalette("", all, projects, closed, [], 63)
    const groups = paletteGroups("Projects", cut, messages)
    const closedGroup = groups.find((g) => g.label === "Closed")
    // The header reads "4 of 63": the cut happened in the store, so the rows in
    // hand cannot report it on their own.
    expect(closedGroup?.rows).toHaveLength(4)
    expect(closedGroup?.total).toBe(63)
  })

  it("never reports a total under the rows it is showing", () => {
    // A stale total from the query before this one must not read as a group
    // that shrank below its own rows.
    const stale = filterPalette("", all, projects, closed, [], 1)
    const closedGroup = paletteGroups("Projects", stale, messages).find((g) => g.label === "Closed")
    expect(closedGroup?.total).toBe(4)
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
    parkedBranch: "feat/relay-inbox",
    closedAt: 1_700_000_200,
    snippet: "",
    truncated: false,
  },
  {
    id: "h2",
    projectId: "p1",
    projectName: "lich",
    projectPath: "/home/u/try/skipo",
    label: "Conpty handle recycling",
    kind: "claude",
    path: "/home/u/wt/lich/conpty",
    // Parked on one branch, sitting on another now: the checkout moved on after
    // the close, which is the disagreement the two fields exist to hold.
    parkedBranch: "fix/conpty-first-try",
    closedAt: 1_700_000_100,
    snippet: "",
    truncated: false,
  },
  {
    id: "h3",
    projectId: "p2",
    projectName: "revu",
    projectPath: "/home/u/try/revu",
    label: "vitest --ui",
    kind: "shell",
    path: "/home/u/wt/revu/gone",
    parkedBranch: "chore/vitest-ui",
    closedAt: 1_700_000_000,
    snippet: "",
    truncated: false,
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

  it("narrows on the parked branch too, which is the one the store matched", () => {
    // h2's checkout has moved since it was parked, so the term the store found
    // it by is not the branch on screen — and the filter must not drop the row
    // the store just answered with.
    expect(filterPalette("first-try", [], [], [], rows).history.map((h) => h.id)).toEqual(["h2"])
    // h3's checkout is gone, so git names no branch at all and the snapshot is
    // the only branch that row has.
    expect(filterPalette("vitest-ui", [], [], [], rows).history.map((h) => h.id)).toEqual(["h3"])
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

  it("keeps a row the store matched on its conversation, whatever the row says", () => {
    // The store found "handle recycling" a megabyte into a conversation and cut
    // one window out of it. Nothing on the row carries the words, so re-testing
    // the query here would drop the row the search just answered with.
    const said = historyRows(
      [{ ...(parked[1] as ClosedSession), snippet: "…the ConPTY handle was recycled…" }],
      parkedBranches,
      new Set(),
    )
    expect(filterPalette("recycled", [], [], [], said).history.map((h) => h.id)).toEqual(["h2"])
    // Two words that matched paragraphs apart: only one of them is in the
    // window, and the row is still the answer.
    expect(filterPalette("recycled zombie", [], [], [], said).history.map((h) => h.id)).toEqual([
      "h2",
    ])
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

  it("says how many matched when the store cut the page", () => {
    // The rows in hand are one page; the number beside them is the whole match,
    // which is what the header turns into "3 of 143".
    const group = paletteGroups("History", results, [], 143)[0]
    expect(group?.rows).toHaveLength(3)
    expect(group?.total).toBe(143)
  })

  it("says what it cannot see yet while the store is still indexing", () => {
    // The note displaces the page count: a list that is missing sessions
    // outright matters more than which slice of a match is on screen.
    expect(paletteGroups("History", results, [], 143, 12)[0]?.note).toBe("indexing 12 sessions")
    expect(paletteGroups("History", results, [], 143, 1)[0]?.note).toBe("indexing 1 session")
    expect(paletteGroups("History", results, [], 143, 0)[0]?.note).toBeUndefined()
  })

  it("reports its own rows when nothing was cut", () => {
    // total === rows.length is what the header reads as "not cut" — and a count
    // that arrived stale, behind the rows, must never claim less than is drawn.
    expect(paletteGroups("History", results, [], 3)[0]?.total).toBe(3)
    expect(paletteGroups("History", results, [], 0)[0]?.total).toBe(3)
  })
})

describe("historyIndexNote", () => {
  const rows = historyRows(parked, parkedBranches, new Set())

  it("says nothing for a session indexed whole, which is nearly all of them", () => {
    expect(historyIndexNote(rows[0] as PaletteHistory)).toBeUndefined()
  })

  it("names how much of a session was indexed when the cap cut it", () => {
    // The size is spelled in the window and the fact in the backend, so this is
    // the line that fails when terminal.indexTextBytes moves and the sentence
    // does not.
    const cut = { ...(rows[0] as PaletteHistory), truncated: true }
    expect(historyIndexNote(cut)).toBe("indexed: newest 8 MB")
  })
})

describe("historyAction", () => {
  const rows = historyRows(parked, parkedBranches, new Set(["/home/u/wt/revu/gone"]))

  it("resumes a row whose checkout is still there and forgets one whose is not", () => {
    expect(historyAction(rows[0] as PaletteHistory)).toBe("resume")
    expect(historyAction(rows[2] as PaletteHistory)).toBe("forget")
  })
})
