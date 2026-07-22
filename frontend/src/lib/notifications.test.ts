import {describe, expect, it} from "vitest"

import {notificationsFrom} from "./notifications"
import type {PendingStatus} from "./session-status-store"
import type {SessionState} from "./sessions"

const projects = [
  {id: "p1", name: "alpha"},
  {id: "p2", name: "beta"},
]

const sessions: SessionState = {
  p1: {
    sessions: [{id: "s1", label: "Fix the bug", kind: "claude"}],
    activeId: "s1",
    nextSeq: 2,
  },
  p2: {
    sessions: [{id: "s2", label: "Write docs", kind: "codex"}],
    activeId: "s2",
    nextSeq: 2,
  },
}

// No project in view: nothing is focused, so every resolved status is kept.
const NONE_ACTIVE = undefined

describe("notificationsFrom", () => {
  it("resolves each pending status to its project and label", () => {
    const pending: PendingStatus[] = [
      {id: "s1", status: "waiting"},
      {id: "s2", status: "done"},
    ]
    expect(notificationsFrom(pending, projects, sessions, NONE_ACTIVE)).toEqual([
      {
        id: "s1",
        status: "waiting",
        projectId: "p1",
        projectName: "alpha",
        sessionLabel: "Fix the bug",
      },
      {
        id: "s2",
        status: "done",
        projectId: "p2",
        projectName: "beta",
        sessionLabel: "Write docs",
      },
    ])
  })

  it("drops a status whose session no longer exists", () => {
    const pending: PendingStatus[] = [
      {id: "s1", status: "waiting"},
      {id: "closed", status: "done"},
    ]
    const result = notificationsFrom(pending, projects, sessions, NONE_ACTIVE)
    expect(result.map((n) => n.id)).toEqual(["s1"])
  })

  // The bug: a turn finishing in the session you are watching should not notify
  // you about itself. The focused session — active session of the active
  // project — is dropped; a non-active session in that same open project stays.
  it("drops the focused session but keeps others in the open project", () => {
    const twoInP1: SessionState = {
      ...sessions,
      p1: {
        sessions: [
          {id: "s1", label: "Fix the bug", kind: "claude"},
          {id: "s1b", label: "Add tests", kind: "claude"},
        ],
        activeId: "s1",
        nextSeq: 3,
      },
    }
    const pending: PendingStatus[] = [
      {id: "s1", status: "done"}, // focused: on screen
      {id: "s1b", status: "waiting"}, // same project, not on screen
      {id: "s2", status: "done"}, // other project
    ]
    const result = notificationsFrom(pending, projects, twoInP1, "p1")
    expect(result.map((n) => n.id)).toEqual(["s1b", "s2"])
  })

  it("returns nothing for an empty queue", () => {
    expect(notificationsFrom([], projects, sessions, NONE_ACTIVE)).toEqual([])
  })
})
