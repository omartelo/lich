import {describe, expect, it} from "vitest"
import {
  isCwdEvent,
  isIdEvent,
  isStatusEvent,
  isTitleEvent,
  shouldToastAttention,
  toSessionStatus,
} from "./session-events"
import {addSession, createProjectSessions, setActiveSession, type SessionState} from "./sessions"

const ACTIVE = "project-active"
const BACKGROUND = "project-background"

// Two projects of two sessions each; "s2"/"b2" are the active session of their
// project, mirroring the shape the provider reads on an attention event.
function buildState(): SessionState {
  let state: SessionState = {
    [ACTIVE]: createProjectSessions("s1"),
    [BACKGROUND]: createProjectSessions("b1"),
  }
  state = addSession(state, ACTIVE, "s2")
  state = addSession(state, BACKGROUND, "b2")
  return state
}

describe("isStatusEvent", () => {
  it("accepts a payload carrying an id and a state", () => {
    expect(isStatusEvent({id: "abc", state: "busy"})).toBe(true)
  })

  it("rejects a payload missing either field or typed wrong", () => {
    expect(isStatusEvent({id: "abc"})).toBe(false)
    expect(isStatusEvent({state: "busy"})).toBe(false)
    expect(isStatusEvent({id: "abc", state: 42})).toBe(false)
    expect(isStatusEvent({id: 1, state: "busy"})).toBe(false)
    expect(isStatusEvent(null)).toBe(false)
    expect(isStatusEvent("busy")).toBe(false)
  })
})

describe("toSessionStatus", () => {
  it("keeps each state the card renders an indicator for", () => {
    expect(toSessionStatus("busy")).toBe("busy")
    expect(toSessionStatus("done")).toBe("done")
    expect(toSessionStatus("waiting")).toBe("waiting")
  })

  it("clears the indicator on the contract's idle", () => {
    expect(toSessionStatus("idle")).toBeNull()
  })

  it("clears the indicator on an unknown state", () => {
    expect(toSessionStatus("thinking")).toBeNull()
    expect(toSessionStatus("")).toBeNull()
  })

  it("clears the indicator on a non-string payload", () => {
    expect(toSessionStatus(undefined)).toBeNull()
    expect(toSessionStatus(null)).toBeNull()
    expect(toSessionStatus(42)).toBeNull()
    expect(toSessionStatus({state: "busy"})).toBeNull()
  })
})

describe("isIdEvent", () => {
  it("accepts a payload carrying a string id", () => {
    expect(isIdEvent({id: "s1"})).toBe(true)
  })

  it("rejects a missing, non-string or non-object payload", () => {
    expect(isIdEvent({})).toBe(false)
    expect(isIdEvent({id: 1})).toBe(false)
    expect(isIdEvent(null)).toBe(false)
    expect(isIdEvent("s1")).toBe(false)
  })
})

describe("isTitleEvent", () => {
  it("accepts a payload carrying a string id and label", () => {
    expect(isTitleEvent({id: "s1", label: "build"})).toBe(true)
  })

  it("rejects a payload missing either half", () => {
    expect(isTitleEvent({id: "s1"})).toBe(false)
    expect(isTitleEvent({label: "build"})).toBe(false)
    expect(isTitleEvent({id: "s1", label: 2})).toBe(false)
    expect(isTitleEvent(null)).toBe(false)
  })
})

describe("isCwdEvent", () => {
  it("accepts a payload carrying a string id and cwd", () => {
    expect(isCwdEvent({id: "s1", cwd: "/home/user"})).toBe(true)
  })

  it("rejects a payload missing either half", () => {
    expect(isCwdEvent({id: "s1"})).toBe(false)
    expect(isCwdEvent({cwd: "/home/user"})).toBe(false)
    expect(isCwdEvent({id: "s1", cwd: 2})).toBe(false)
    expect(isCwdEvent(null)).toBe(false)
  })
})

describe("shouldToastAttention", () => {
  it("stays silent for the session already in focus", () => {
    expect(shouldToastAttention(buildState(), "s2", ACTIVE)).toBe(false)
  })

  it("toasts a background session of the active project", () => {
    expect(shouldToastAttention(buildState(), "s1", ACTIVE)).toBe(true)
  })

  it("toasts the active session of a background project", () => {
    expect(shouldToastAttention(buildState(), "b2", ACTIVE)).toBe(true)
  })

  it("toasts a background session of a background project", () => {
    expect(shouldToastAttention(buildState(), "b1", ACTIVE)).toBe(true)
  })

  it("toasts every session while no project is focused", () => {
    expect(shouldToastAttention(buildState(), "s2", undefined)).toBe(true)
  })

  it("toasts the previously focused session once focus moves away", () => {
    const state = setActiveSession(buildState(), ACTIVE, "s1")
    expect(shouldToastAttention(state, "s2", ACTIVE)).toBe(true)
    expect(shouldToastAttention(state, "s1", ACTIVE)).toBe(false)
  })

  it("stays silent for a session no project holds", () => {
    expect(shouldToastAttention(buildState(), "ghost", ACTIVE)).toBe(false)
  })

  it("stays silent for an empty state", () => {
    expect(shouldToastAttention({}, "s1", ACTIVE)).toBe(false)
  })
})
