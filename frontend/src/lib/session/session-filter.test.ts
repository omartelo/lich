import { describe, expect, it } from "vitest"
import {
  filterSessions,
  noMatchNotice,
  phaseCounts,
  phaseOf,
  type SessionPhase,
} from "./session-filter"
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

const ANY_PHASE: ReadonlySet<SessionPhase> = new Set()
const allIdle = (): SessionPhase => "idle"

const PHASES: Record<string, SessionPhase> = {
  s1: "waiting",
  s2: "running",
  s3: "unread",
  s4: "idle",
  s5: "running",
  s6: "idle",
}
const phaseOfFixture = (id: string): SessionPhase => PHASES[id]

describe("filterSessions", () => {
  it("hands the list straight back for a blank query", () => {
    const filter = filterSessions(sessions, "   ", PROJECT, "s3", ANY_PHASE, allIdle)
    expect(filter.sessions).toBe(sessions)
    expect(filter.matched).toBe(true)
  })

  it("matches labels across groups and keeps the stored order", () => {
    const filter = filterSessions(sessions, "relay", PROJECT, "s3", ANY_PHASE, allIdle)
    // s5 and s6 ride in on their checkout path, which carries "auth-relay".
    expect(ids(filter.sessions)).toEqual(["s1", "s3", "s5", "s6"])
    expect(filter.matched).toBe(true)
  })

  it("narrows to a checkout when the query is a worktree name", () => {
    const filter = filterSessions(sessions, "auth-relay", PROJECT, "s5", ANY_PHASE, allIdle)
    expect(ids(filter.sessions)).toEqual(["s5", "s6"])
  })

  it("matches the project's own path for a session without one", () => {
    const filter = filterSessions(sessions, "code/lich", PROJECT, "s3", ANY_PHASE, allIdle)
    expect(ids(filter.sessions)).toEqual(["s1", "s2", "s3", "s4"])
  })

  it("keeps the active session when it does not match", () => {
    const filter = filterSessions(sessions, "relay", PROJECT, "s4", ANY_PHASE, allIdle)
    expect(ids(filter.sessions)).toEqual(["s1", "s3", "s4", "s5", "s6"])
    expect(filter.matched).toBe(true)
  })

  it("reports no match even though the active session is still drawn", () => {
    const filter = filterSessions(sessions, "webgl", PROJECT, "s4", ANY_PHASE, allIdle)
    expect(ids(filter.sessions)).toEqual(["s4"])
    expect(filter.matched).toBe(false)
  })

  it("draws nothing at all when nothing matches and no session is active", () => {
    const filter = filterSessions(sessions, "webgl", PROJECT, "", ANY_PHASE, allIdle)
    expect(filter.sessions).toEqual([])
    expect(filter.matched).toBe(false)
  })

  it("is case-insensitive and splits the query into tokens", () => {
    expect(
      ids(filterSessions(sessions, "TICKET relay", PROJECT, "", ANY_PHASE, allIdle).sessions),
    ).toEqual(["s3"])
  })
})

describe("phaseOf", () => {
  it("sorts every status the card renders into the chip it lands under", () => {
    expect(phaseOf("waiting", false)).toBe("waiting")
    expect(phaseOf("busy", false)).toBe("running")
    expect(phaseOf("compacting", false)).toBe("running")
    expect(phaseOf("done", true)).toBe("unread")
  })

  it("files a read turn and a session with no status under idle", () => {
    expect(phaseOf("done", false)).toBe("idle")
    expect(phaseOf(null, false)).toBe("idle")
  })
})

describe("filterSessions by phase", () => {
  it("keeps the sessions in any picked phase", () => {
    const filter = filterSessions(
      sessions,
      "",
      PROJECT,
      "",
      new Set<SessionPhase>(["waiting", "unread"]),
      phaseOfFixture,
    )
    expect(ids(filter.sessions)).toEqual(["s1", "s3"])
    expect(filter.matched).toBe(true)
  })

  it("narrows by the query and the phase together", () => {
    const filter = filterSessions(
      sessions,
      "relay",
      PROJECT,
      "",
      new Set<SessionPhase>(["running"]),
      phaseOfFixture,
    )
    expect(ids(filter.sessions)).toEqual(["s5"])
  })

  it("keeps the active session whatever phase it is in", () => {
    const filter = filterSessions(
      sessions,
      "",
      PROJECT,
      "s6",
      new Set<SessionPhase>(["waiting"]),
      phaseOfFixture,
    )
    expect(ids(filter.sessions)).toEqual(["s1", "s6"])
  })

  it("reports no match when only the active session is left", () => {
    const filter = filterSessions(
      sessions,
      "patch",
      PROJECT,
      "s4",
      new Set<SessionPhase>(["waiting"]),
      phaseOfFixture,
    )
    expect(ids(filter.sessions)).toEqual(["s4"])
    expect(filter.matched).toBe(false)
  })
})

describe("phaseCounts", () => {
  it("counts every session per phase for a blank query", () => {
    expect(phaseCounts(sessions, "", PROJECT, phaseOfFixture)).toEqual({
      waiting: 1,
      running: 2,
      unread: 1,
      idle: 2,
    })
  })

  it("counts only the sessions the query matches", () => {
    expect(phaseCounts(sessions, "relay", PROJECT, phaseOfFixture)).toEqual({
      waiting: 1,
      running: 1,
      unread: 1,
      idle: 1,
    })
  })
})

describe("noMatchNotice", () => {
  it("quotes the query alone", () => {
    expect(noMatchNotice(" webgl ", ANY_PHASE)).toBe(
      "No sessions match “webgl”. The active session stays.",
    )
  })

  it("names the picked phases in chip order", () => {
    expect(noMatchNotice("", new Set<SessionPhase>(["unread", "waiting"]))).toBe(
      "No waiting or unread sessions. The active session stays.",
    )
  })

  it("names the phases and quotes the query together", () => {
    expect(noMatchNotice("webgl", new Set<SessionPhase>(["running"]))).toBe(
      "No running sessions match “webgl”. The active session stays.",
    )
  })
})
