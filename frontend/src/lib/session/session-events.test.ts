import { describe, expect, it } from "vitest"
import {
  decideDesktopNotice,
  decideStatusNotice,
  isAgentEvent,
  isCwdEvent,
  isIdEvent,
  isStatusEvent,
  isTitleEvent,
  isUsageEvent,
  shouldToastAttention,
  toClosedSession,
  toOpenedSession,
  toSessionStatus,
} from "./session-events"
import { addSession, setActiveSession, type SessionState } from "./sessions"

const ACTIVE = "project-active"
const BACKGROUND = "project-background"

// Two projects of two sessions each; "s2"/"b2" are the active session of their
// project, mirroring the shape the provider reads on an attention event.
function buildState(): SessionState {
  let state: SessionState = addSession({}, ACTIVE, "s1")
  state = addSession(state, BACKGROUND, "b1")
  state = addSession(state, ACTIVE, "s2")
  state = addSession(state, BACKGROUND, "b2")
  return state
}

describe("isStatusEvent", () => {
  it("accepts a payload carrying an id and a state", () => {
    expect(isStatusEvent({ id: "abc", state: "busy" })).toBe(true)
  })

  it("rejects a payload missing either field or typed wrong", () => {
    expect(isStatusEvent({ id: "abc" })).toBe(false)
    expect(isStatusEvent({ state: "busy" })).toBe(false)
    expect(isStatusEvent({ id: "abc", state: 42 })).toBe(false)
    expect(isStatusEvent({ id: 1, state: "busy" })).toBe(false)
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
    expect(toSessionStatus({ state: "busy" })).toBeNull()
  })
})

describe("isIdEvent", () => {
  it("accepts a payload carrying a string id", () => {
    expect(isIdEvent({ id: "s1" })).toBe(true)
  })

  it("rejects a missing, non-string or non-object payload", () => {
    expect(isIdEvent({})).toBe(false)
    expect(isIdEvent({ id: 1 })).toBe(false)
    expect(isIdEvent(null)).toBe(false)
    expect(isIdEvent("s1")).toBe(false)
  })
})

describe("isTitleEvent", () => {
  it("accepts a payload carrying a string id and label", () => {
    expect(isTitleEvent({ id: "s1", label: "build" })).toBe(true)
  })

  it("rejects a payload missing either half", () => {
    expect(isTitleEvent({ id: "s1" })).toBe(false)
    expect(isTitleEvent({ label: "build" })).toBe(false)
    expect(isTitleEvent({ id: "s1", label: 2 })).toBe(false)
    expect(isTitleEvent(null)).toBe(false)
  })
})

describe("isCwdEvent", () => {
  it("accepts a payload carrying a string id and cwd", () => {
    expect(isCwdEvent({ id: "s1", cwd: "/home/user" })).toBe(true)
  })

  it("rejects a payload missing either half", () => {
    expect(isCwdEvent({ id: "s1" })).toBe(false)
    expect(isCwdEvent({ cwd: "/home/user" })).toBe(false)
    expect(isCwdEvent({ id: "s1", cwd: 2 })).toBe(false)
    expect(isCwdEvent(null)).toBe(false)
  })
})

describe("isAgentEvent", () => {
  it("accepts a payload carrying a string id and agent, empty included", () => {
    expect(isAgentEvent({ id: "s1", agent: "claude" })).toBe(true)
    expect(isAgentEvent({ id: "s1", agent: "" })).toBe(true)
  })

  it("rejects a payload missing either half", () => {
    expect(isAgentEvent({ id: "s1" })).toBe(false)
    expect(isAgentEvent({ agent: "claude" })).toBe(false)
    expect(isAgentEvent({ id: "s1", agent: 2 })).toBe(false)
    expect(isAgentEvent(null)).toBe(false)
  })
})

describe("isUsageEvent", () => {
  const full = {
    id: "s1",
    percent: 42,
    tokens: 84000,
    window: 200000,
    model: "claude-opus-4-8",
    effort: "xhigh",
  }

  it("accepts a payload carrying id, percent, tokens, window, model and effort", () => {
    expect(isUsageEvent(full)).toBe(true)
    expect(isUsageEvent({ ...full, percent: 0, tokens: 0, effort: "" })).toBe(true)
  })

  it("rejects a payload missing a field or typed wrong", () => {
    expect(isUsageEvent({ id: "s1", percent: 42, tokens: 84000, window: 200000, model: "x" })).toBe(
      false,
    )
    expect(isUsageEvent({ ...full, window: undefined })).toBe(false)
    expect(isUsageEvent({ ...full, model: 5 })).toBe(false)
    expect(isUsageEvent({ ...full, effort: 3 })).toBe(false)
    expect(isUsageEvent({ ...full, id: undefined })).toBe(false)
    expect(isUsageEvent({ ...full, percent: "42" })).toBe(false)
    expect(isUsageEvent(null)).toBe(false)
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

describe("decideDesktopNotice", () => {
  it("stays out of the way while the user is at the window", () => {
    expect(decideDesktopNotice(true, true)).toBe("none")
    expect(decideDesktopNotice(true, null)).toBe("none")
    expect(decideDesktopNotice(true, false)).toBe("none")
  })

  it("notifies once the user has opted in and left the window", () => {
    expect(decideDesktopNotice(false, true)).toBe("notify")
  })

  it("puts the question on the first report that would have notified", () => {
    expect(decideDesktopNotice(false, null)).toBe("ask")
  })

  it("keeps quiet for a refusal instead of asking again", () => {
    expect(decideDesktopNotice(false, false)).toBe("none")
  })
})

describe("decideStatusNotice", () => {
  const BOTH_ON = { attention: true, finishedTurn: true }
  const BOTH_OFF = { attention: false, finishedTurn: false }
  const FRESH = null // no previous status: nothing to collapse against

  it("stays out of the way while the user is at the window", () => {
    expect(decideStatusNotice("waiting", FRESH, true, BOTH_ON)).toBe("none")
    expect(decideStatusNotice("done", FRESH, true, BOTH_ON)).toBe("none")
  })

  it("notifies for each channel the user turned on", () => {
    expect(decideStatusNotice("waiting", FRESH, false, BOTH_ON)).toBe("notify")
    expect(decideStatusNotice("done", "busy", false, BOTH_ON)).toBe("notify")
  })

  it("stays silent for each channel the user left off", () => {
    expect(decideStatusNotice("waiting", FRESH, false, BOTH_OFF)).toBe("none")
    expect(decideStatusNotice("done", "busy", false, BOTH_OFF)).toBe("none")
  })

  // The switches are independent: neither channel rides the other's answer.
  it("keeps the two channels apart", () => {
    const onlyAttention = { attention: true, finishedTurn: false }
    expect(decideStatusNotice("waiting", FRESH, false, onlyAttention)).toBe("notify")
    expect(decideStatusNotice("done", "busy", false, onlyAttention)).toBe("none")

    const onlyFinished = { attention: false, finishedTurn: true }
    expect(decideStatusNotice("waiting", FRESH, false, onlyFinished)).toBe("none")
    expect(decideStatusNotice("done", "busy", false, onlyFinished)).toBe("notify")
  })

  it("says nothing about a session that is merely running", () => {
    expect(decideStatusNotice("busy", "waiting", false, BOTH_ON)).toBe("none")
    expect(decideStatusNotice(null, "busy", false, BOTH_ON)).toBe("none")
  })

  // The dialog belongs to the blocked-on-you channel alone: a finished turn with
  // the opt-in unanswered must not put a question the user never asked for.
  it("only ever puts the question for a blocked session", () => {
    const unanswered = { attention: null, finishedTurn: true }
    expect(decideStatusNotice("waiting", FRESH, false, unanswered)).toBe("ask")
    expect(decideStatusNotice("done", "busy", false, unanswered)).toBe("notify")
  })

  it("does not notify twice for the same finished turn", () => {
    expect(decideStatusNotice("done", "done", false, BOTH_ON)).toBe("none")
    // A turn that ran again is a new one, so it announces itself again.
    expect(decideStatusNotice("done", "busy", false, BOTH_ON)).toBe("notify")
  })

  // The toast's contract, which the desktop channel shares: a second prompt is
  // a second thing to answer, so a repeat "waiting" is not collapsed.
  it("notifies again for a repeated waiting report", () => {
    expect(decideStatusNotice("waiting", "waiting", false, BOTH_ON)).toBe("notify")
  })
})

describe("toOpenedSession", () => {
  const payload = {
    id: "agent-1",
    projectId: "p1",
    project: "lich",
    label: "auth-fix",
    name: "auth-fix-agen",
    kind: "claude",
    path: "/wt/auth-fix",
    nextSeq: 5,
    originSessionId: "s1",
    originLabel: "planner",
  }

  it("narrows a session opened outside the window", () => {
    expect(toOpenedSession(payload)).toEqual({
      id: "agent-1",
      projectId: "p1",
      label: "auth-fix",
      kind: "claude",
      path: "/wt/auth-fix",
      nextSeq: 5,
      originSessionId: "s1",
      originLabel: "planner",
    })
  })

  // A session opened from the window, or by `lich open` from a plain shell, has
  // no origin at all — absent is the normal case, not a malformed payload.
  it("keeps a session that nobody delegated", () => {
    const opened = toOpenedSession({ ...payload, originSessionId: undefined, originLabel: 4 })
    expect(opened?.originSessionId).toBe("")
    expect(opened?.originLabel).toBe("")
  })

  it("keeps a session in the project's own directory", () => {
    const opened = toOpenedSession({ ...payload, path: "", label: "Session 4" })
    expect(opened?.path).toBe("")
    expect(opened?.label).toBe("Session 4")
  })

  it("rejects a payload with nothing to place the card by", () => {
    expect(toOpenedSession({ ...payload, id: undefined })).toBeNull()
    expect(toOpenedSession({ ...payload, projectId: "" })).toBeNull()
    expect(toOpenedSession({ ...payload, label: 4 })).toBeNull()
    expect(toOpenedSession(null)).toBeNull()
  })

  // A provider a newer backend knows and this build cannot spawn: the row is
  // written either way and the next load reads it, so dropping the card costs
  // less than adopting one whose terminal can never start.
  it("rejects a kind this build does not know", () => {
    expect(toOpenedSession({ ...payload, kind: "gemini" })).toBeNull()
  })
})

describe("toClosedSession", () => {
  it("narrows a session closed outside the window", () => {
    expect(toClosedSession({ id: "s2", projectId: "p1", activeId: "s3" })).toEqual({
      id: "s2",
      projectId: "p1",
      activeId: "s3",
    })
  })

  // An empty active session is the project having none left, not a missing
  // field: the last card of a project closes to nothing.
  it("keeps an empty active session", () => {
    expect(toClosedSession({ id: "s2", projectId: "p1", activeId: "" })?.activeId).toBe("")
    expect(toClosedSession({ id: "s2", projectId: "p1" })?.activeId).toBe("")
  })

  it("rejects a payload with nothing to place the card by", () => {
    expect(toClosedSession({ projectId: "p1" })).toBeNull()
    expect(toClosedSession({ id: "s2", projectId: "" })).toBeNull()
    expect(toClosedSession(null)).toBeNull()
  })
})
