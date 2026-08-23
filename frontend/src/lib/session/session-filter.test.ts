import { describe, expect, it } from "vitest"
import { filterSessions } from "./session-filter"
import type { Session } from "./sessions"

const PROJECT = "/home/u/code/lich"
const WORKTREE = "/home/u/.local/share/lich/worktrees/lich/feat/auth-relay"

const sessions: Session[] = [
  { id: "s1", label: "Relay inbox nudge", kind: "claude", pinned: true },
  { id: "s2", label: "Patch notes", kind: "codex", pinned: true },
  { id: "s3", label: "Relay ticket lifecycle", kind: "claude" },
  { id: "s4", label: "shell", kind: "shell" },
  { id: "s5", label: "Wire the auth token", kind: "claude", path: WORKTREE },
  { id: "s6", label: "Round-trip tests", kind: "codex", path: WORKTREE },
]

const ids = (list: Session[]) => list.map((session) => session.id)

describe("filterSessions", () => {
  it("hands the list straight back for a blank query", () => {
    const filter = filterSessions(sessions, "   ", PROJECT, "s3")
    expect(filter.sessions).toBe(sessions)
    expect(filter.matched).toBe(true)
  })

  it("matches labels across groups and keeps the stored order", () => {
    const filter = filterSessions(sessions, "relay", PROJECT, "s3")
    // s5 and s6 ride in on their checkout path, which carries "auth-relay".
    expect(ids(filter.sessions)).toEqual(["s1", "s3", "s5", "s6"])
    expect(filter.matched).toBe(true)
  })

  it("narrows to a checkout when the query is a worktree name", () => {
    const filter = filterSessions(sessions, "auth-relay", PROJECT, "s5")
    expect(ids(filter.sessions)).toEqual(["s5", "s6"])
  })

  it("matches the project's own path for a session without one", () => {
    const filter = filterSessions(sessions, "code/lich", PROJECT, "s3")
    expect(ids(filter.sessions)).toEqual(["s1", "s2", "s3", "s4"])
  })

  it("keeps the active session when it does not match", () => {
    const filter = filterSessions(sessions, "relay", PROJECT, "s4")
    expect(ids(filter.sessions)).toEqual(["s1", "s3", "s4", "s5", "s6"])
    expect(filter.matched).toBe(true)
  })

  it("reports no match even though the active session is still drawn", () => {
    const filter = filterSessions(sessions, "webgl", PROJECT, "s4")
    expect(ids(filter.sessions)).toEqual(["s4"])
    expect(filter.matched).toBe(false)
  })

  it("draws nothing at all when nothing matches and no session is active", () => {
    const filter = filterSessions(sessions, "webgl", PROJECT, "")
    expect(filter.sessions).toEqual([])
    expect(filter.matched).toBe(false)
  })

  it("is case-insensitive and splits the query into tokens", () => {
    expect(ids(filterSessions(sessions, "TICKET relay", PROJECT, "").sessions)).toEqual(["s3"])
  })
})
