import { describe, expect, it } from "vitest"
import {
  activeSessionId,
  addSession,
  closeSession,
  createProjectSessions,
  isLastWorktreeSession,
  isSessionKind,
  projectOfSession,
  removeProject,
  renameSession,
  reorderSessions,
  restoreSession,
  resumableSession,
  sessionsOf,
  setActiveSession,
  type Session,
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

describe("isSessionKind", () => {
  it("accepts every provider kind and the shell", () => {
    for (const kind of ["claude", "codex", "opencode", "crush", "shell"]) {
      expect(isSessionKind(kind)).toBe(true)
    }
  })

  it("rejects unknown strings", () => {
    for (const kind of ["", "bash", "gpt", "Claude", "worktree"]) {
      expect(isSessionKind(kind)).toBe(false)
    }
  })
})

// withClaudeSession stamps a restored session's Claude session id onto the
// state, the way hydration from the store does.
function withClaudeSession(
  state: SessionState,
  sessionId: string,
  providerSessionId: string,
  kind: "claude" | "shell" = "claude",
): SessionState {
  return {
    ...state,
    [P]: {
      ...state[P],
      sessions: state[P].sessions.map((s) =>
        s.id === sessionId ? { ...s, kind, providerSessionId } : s,
      ),
    },
  }
}

describe("createProjectSessions", () => {
  it("seeds one active claude session labeled Session 1", () => {
    const project = createProjectSessions("s1")
    expect(project.sessions).toEqual([{ id: "s1", label: "Session 1", kind: "claude" }])
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

  it("records the requested kind, defaulting to claude", () => {
    const state = addSession(addSession({}, P, "s1"), P, "s2", "shell")
    expect(sessionsOf(state, P).map((s) => s.kind)).toEqual(["claude", "shell"])
  })

  it("records a worktree path and labels the session after it", () => {
    const state = addSession(buildState(1), P, "s2", "claude", "/wt/lucky-otter", "lucky-otter")
    const created = sessionsOf(state, P)[1]
    expect(created).toEqual({
      id: "s2",
      label: "lucky-otter",
      kind: "claude",
      path: "/wt/lucky-otter",
    })
  })

  it("omits path and keeps the sequential label when no worktree is given", () => {
    const created = sessionsOf(addSession(buildState(1), P, "s2"), P)[1]
    expect(created.path).toBeUndefined()
    expect(created.label).toBe("Session 2")
  })

  it("preserves path through close and rename of siblings", () => {
    let state = addSession(buildState(1), P, "s2", "claude", "/wt/x", "x")
    state = addSession(state, P, "s3")
    state = closeSession(state, P, "s3")
    state = renameSession(state, P, "s1", "renamed")
    expect(sessionsOf(state, P).find((s) => s.id === "s2")?.path).toBe("/wt/x")
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

  it("empties the project but preserves nextSeq", () => {
    const state = closeSession(buildState(1), P, "s1")
    expect(sessionsOf(state, P)).toHaveLength(0)
    expect(activeSessionId(state, P)).toBe("")
    // The next session keeps counting up instead of reusing "Session 1".
    const reopened = addSession(state, P, "s2")
    expect(sessionsOf(reopened, P)[0].label).toBe("Session 2")
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

describe("renameSession", () => {
  it("relabels the target session and leaves siblings untouched", () => {
    const state = renameSession(buildState(2), P, "s1", "build")
    expect(sessionsOf(state, P).map((s) => s.label)).toEqual([
      "build",
      "Session 2",
    ])
  })

  it("does not mutate the input state", () => {
    const before = buildState(1)
    renameSession(before, P, "s1", "build")
    expect(sessionsOf(before, P)[0].label).toBe("Session 1")
  })

  it("ignores unknown project or session ids", () => {
    const state = buildState(2)
    expect(renameSession(state, "nope", "s1", "x")).toBe(state)
    expect(renameSession(state, P, "ghost", "x")).toBe(state)
  })
})

describe("projectOfSession", () => {
  it("returns the id of the project owning the session", () => {
    const state = buildState(3)
    expect(projectOfSession(state, "s2")).toBe(P)
  })

  it("returns empty string when no project holds the session", () => {
    expect(projectOfSession(buildState(2), "ghost")).toBe("")
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

describe("reorderSessions", () => {
  it("rearranges the sessions to the given id order", () => {
    const state = buildState(3)
    const next = reorderSessions(state, P, ["s3", "s1", "s2"])
    expect(sessionsOf(next, P).map((s) => s.id)).toEqual(["s3", "s1", "s2"])
  })

  it("leaves the active session and the label counter alone", () => {
    const state = buildState(3)
    const next = reorderSessions(state, P, ["s3", "s2", "s1"])
    expect(activeSessionId(next, P)).toBe(activeSessionId(state, P))
    expect(next[P].nextSeq).toBe(state[P].nextSeq)
  })

  it("ignores an unknown project", () => {
    const state = buildState(2)
    expect(reorderSessions(state, "nope", ["s2", "s1"])).toBe(state)
  })

  // A close racing the drag leaves the dragged order naming a session that is
  // gone; persisting it would drop a survivor from the list.
  it("ignores an order that no longer matches the session set", () => {
    const state = buildState(3)
    expect(reorderSessions(state, P, ["s3", "s1"])).toBe(state)
    expect(reorderSessions(state, P, ["s3", "s2", "s1", "ghost"])).toBe(state)
  })

  it("does not mutate the input state", () => {
    const state = buildState(3)
    reorderSessions(state, P, ["s3", "s2", "s1"])
    expect(sessionsOf(state, P).map((s) => s.id)).toEqual(["s1", "s2", "s3"])
  })
})

describe("resumableSession", () => {
  it("returns a restored claude session carrying a claude session id", () => {
    const state = withClaudeSession(buildState(2), "s1", "claude-abc")
    expect(resumableSession(state, P, "s1")).toMatchObject({
      id: "s1",
      providerSessionId: "claude-abc",
    })
  })

  // A session created in this run has nothing to resume; only hydration from
  // the store sets the id.
  it("returns null for a session without a claude session id", () => {
    expect(resumableSession(buildState(2), P, "s1")).toBeNull()
  })

  // Running Claude Code by hand inside a shell session lets the SessionStart
  // hook stamp an id on its row — the shell still cannot reopen it.
  it("returns null for a shell session even with a claude session id", () => {
    const state = withClaudeSession(buildState(2), "s1", "claude-abc", "shell")
    expect(resumableSession(state, P, "s1")).toBeNull()
  })

  it("returns null for unknown project and session ids", () => {
    const state = withClaudeSession(buildState(2), "s1", "claude-abc")
    expect(resumableSession(state, "nope", "s1")).toBeNull()
    expect(resumableSession(state, P, "ghost")).toBeNull()
  })
})

describe("restoreSession", () => {
  const parked = {
    id: "wt2",
    label: "swift-rabbit",
    kind: "claude" as const,
    path: "/wt/swift-rabbit",
    providerSessionId: "claude-abc",
  }

  it("re-adds the session, focuses it, and keeps its claude session id", () => {
    const state = restoreSession(buildState(2), P, parked)
    const sessions = sessionsOf(state, P)
    expect(sessions.map((s) => s.id)).toEqual(["s1", "s2", "wt2"])
    expect(activeSessionId(state, P)).toBe("wt2")
    expect(sessions[2].providerSessionId).toBe("claude-abc")
  })

  it("does not advance the label counter (not a new numbered session)", () => {
    const before = buildState(2)[P].nextSeq
    const state = restoreSession(buildState(2), P, parked)
    expect(state[P].nextSeq).toBe(before)
  })

  it("just focuses an id already present instead of duplicating it", () => {
    const seeded = restoreSession(buildState(1), P, parked)
    const again = restoreSession(setActiveSession(seeded, P, "s1"), P, parked)
    expect(sessionsOf(again, P).map((s) => s.id)).toEqual(["s1", "wt2"])
    expect(activeSessionId(again, P)).toBe("wt2")
  })

  it("ignores an unknown project", () => {
    const state = buildState(1)
    expect(restoreSession(state, "nope", parked)).toBe(state)
  })
})

describe("isLastWorktreeSession", () => {
  const wt = (id: string, path?: string): Session => ({
    id,
    label: id,
    kind: "shell",
    ...(path ? { path } : {}),
  })

  it("is false for a project-rooted (pathless) session", () => {
    const s = wt("s1")
    expect(isLastWorktreeSession([s], s)).toBe(false)
  })

  it("is true when the session is the only occupant of its worktree", () => {
    const s = wt("s1", "/wt/a")
    expect(isLastWorktreeSession([s, wt("s2", "/wt/b")], s)).toBe(true)
  })

  it("is false while another session shares the same worktree path", () => {
    const s = wt("s1", "/wt/a")
    expect(isLastWorktreeSession([s, wt("s2", "/wt/a")], s)).toBe(false)
  })
})
