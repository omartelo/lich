import { describe, expect, it } from "vitest"
import {
  activeSessionId,
  addSession,
  closeSession,
  createProjectSessions,
  removeProject,
  sessionsOf,
  setActiveSession,
  type SessionState,
} from "./sessions"

const P = "project-1"

// buildState creates a project with `n` sessions (ids "s1".."sn"), the last one
// active.
function buildState(n: number): SessionState {
  let state: SessionState = { [P]: createProjectSessions("s1") }
  for (let i = 2; i <= n; i++) {
    state = addSession(state, P, `s${i}`)
  }
  return state
}

describe("createProjectSessions", () => {
  it("seeds one active session labeled Session 1", () => {
    const project = createProjectSessions("s1")
    expect(project.sessions).toEqual([{ id: "s1", label: "Session 1" }])
    expect(project.activeId).toBe("s1")
    expect(project.nextSeq).toBe(2)
  })
})

describe("addSession", () => {
  it("creates the project entry when absent", () => {
    const state = addSession({}, P, "s1")
    expect(sessionsOf(state, P)).toHaveLength(1)
    expect(activeSessionId(state, P)).toBe("s1")
  })

  it("appends, focuses the new session and advances the label sequence", () => {
    const state = addSession(buildState(1), P, "s2")
    expect(sessionsOf(state, P).map((s) => s.label)).toEqual([
      "Session 1",
      "Session 2",
    ])
    expect(activeSessionId(state, P)).toBe("s2")
  })

  it("does not mutate the input state", () => {
    const before = buildState(1)
    addSession(before, P, "s2")
    expect(sessionsOf(before, P)).toHaveLength(1)
  })
})

describe("closeSession", () => {
  it("removes a non-active session and keeps the active one", () => {
    const state = closeSession(buildState(3), P, "s1") // active is s3
    expect(sessionsOf(state, P).map((s) => s.id)).toEqual(["s2", "s3"])
    expect(activeSessionId(state, P)).toBe("s3")
  })

  it("moves focus to the slot filler when the active session closes", () => {
    // s1, s2, s3 with s2 active → closing s2 focuses s3 (fills index 1).
    let state = buildState(3)
    state = setActiveSession(state, P, "s2")
    state = closeSession(state, P, "s2")
    expect(activeSessionId(state, P)).toBe("s3")
  })

  it("falls back to the previous session when the last one closes", () => {
    const state = closeSession(buildState(3), P, "s3") // active is s3 (last)
    expect(activeSessionId(state, P)).toBe("s2")
  })

  it("empties the project but preserves nextSeq for recreate", () => {
    const state = closeSession(buildState(1), P, "s1")
    expect(sessionsOf(state, P)).toHaveLength(0)
    expect(activeSessionId(state, P)).toBe("")
    // A recreate keeps counting up instead of reusing "Session 1".
    const recreated = addSession(state, P, "s2")
    expect(sessionsOf(recreated, P)[0].label).toBe("Session 2")
  })

  it("ignores unknown project or session ids", () => {
    const state = buildState(2)
    expect(closeSession(state, "nope", "s1")).toBe(state)
    expect(closeSession(state, P, "ghost")).toBe(state)
  })
})

describe("setActiveSession", () => {
  it("focuses an existing session", () => {
    const state = setActiveSession(buildState(3), P, "s1")
    expect(activeSessionId(state, P)).toBe("s1")
  })

  it("ignores unknown ids", () => {
    const state = buildState(2)
    expect(setActiveSession(state, P, "ghost")).toBe(state)
  })
})

describe("removeProject", () => {
  it("drops the project and its sessions", () => {
    const state = removeProject(buildState(2), P)
    expect(sessionsOf(state, P)).toHaveLength(0)
    expect(P in state).toBe(false)
  })

  it("is a no-op for an unknown project", () => {
    const state = buildState(1)
    expect(removeProject(state, "nope")).toBe(state)
  })
})
