import { describe, expect, it } from "vitest"
import { filterPalette, matchesQuery, paletteSessions } from "./command-palette"
import type { Project } from "./api-types"
import type { SessionState } from "./sessions"

const projects: Project[] = [
  { id: "p1", name: "lich", path: "/home/u/try/skipo" },
  { id: "p2", name: "revu", path: "/home/u/try/revu" },
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
    expect(filterPalette("frontend", all, projects).sessions.map((s) => s.sessionId)).toEqual(["s2"])
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
})
