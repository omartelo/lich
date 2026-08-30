import { describe, expect, it } from "vitest"
import {
  activeSessionId,
  activeTarget,
  addSession,
  adoptSession,
  closeSession,
  dropClosedSession,
  groupByWorktree,
  hasSession,
  isLastWorktreeSession,
  isSessionKind,
  neighborSessionId,
  orderGroups,
  PINNED_GROUP_KEY,
  projectOfSession,
  ROOT_GROUP_KEY,
  removeProject,
  renameSession,
  reorderSessions,
  reorderSubset,
  restoreSession,
  resumableSession,
  sessionOrigin,
  sessionsOf,
  setActiveSession,
  setSessionEntrypoint,
  setSessionPinned,
  setSessionSandboxed,
  sidebarGroups,
  type Session,
  type SessionKind,
  type SessionState,
} from "./sessions"

const P = "project-1"

// buildState creates a project with `n` sessions (ids "s1".."sn"), the last one
// active.
function buildState(n: number): SessionState {
  let state: SessionState = addSession({}, P, "s1")
  for (let i = 2; i <= n; i++) {
    state = addSession(state, P, `s${i}`)
  }
  return state
}

describe("isSessionKind", () => {
  it("accepts every provider kind and the shell", () => {
    for (const kind of [
      "claude",
      "codex",
      "antigravity",
      "opencode",
      "omp",
      "crush",
      "cursor",
      "shell",
    ]) {
      expect(isSessionKind(kind)).toBe(true)
    }
  })

  it("rejects unknown strings", () => {
    for (const kind of ["", "bash", "gpt", "Claude", "worktree"]) {
      expect(isSessionKind(kind)).toBe(false)
    }
  })
})

// withClaudeSession stamps a restored session's provider session id onto the
// state, the way hydration from the store does.
function withClaudeSession(
  state: SessionState,
  sessionId: string,
  providerSessionId: string,
  kind: SessionKind = "claude",
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

describe("addSession", () => {
  it("creates the project entry when absent, as one active Session 1", () => {
    const state = addSession({}, P, "s1")
    expect(sessionsOf(state, P)).toEqual([{ id: "s1", label: "Session 1", kind: "claude" }])
    expect(activeSessionId(state, P)).toBe("s1")
    expect(state[P].nextSeq).toBe(2)
  })

  // The first session of a project used to take a different code path from
  // every later one, and that path ignored both arguments: a worktree opened
  // as a project's first session lost its checkout and its name, spawning in
  // the project root while the store recorded the worktree.
  it("keeps the worktree path and label when it creates the project entry", () => {
    const created = sessionsOf(
      addSession({}, P, "s1", "claude", "/wt/lucky-otter", "lucky-otter"),
      P,
    )[0]
    expect(created).toEqual({
      id: "s1",
      label: "lucky-otter",
      kind: "claude",
      path: "/wt/lucky-otter",
    })
  })

  it("appends, focuses the new session and advances the label sequence", () => {
    const state = addSession(buildState(1), P, "s2")
    expect(sessionsOf(state, P).map((s) => s.label)).toEqual(["Session 1", "Session 2"])
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
    expect(sessionsOf(state, P).map((s) => s.label)).toEqual(["build", "Session 2"])
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

describe("setSessionEntrypoint", () => {
  // A terminal beside an agent session, which is the shape every assertion here
  // is about: only the terminal can carry a command.
  const withTerminal = () => addSession(buildState(1), P, "t1", "shell")

  it("records the command and names the card after it while lich owns the name", () => {
    const state = setSessionEntrypoint(withTerminal(), P, "t1", "lazygit", true)
    const terminal = sessionsOf(state, P).find((s) => s.id === "t1")
    expect(terminal?.entrypoint).toBe("lazygit")
    expect(terminal?.label).toBe("lazygit")
  })

  it("leaves a name the user chose alone", () => {
    const named = renameSession(withTerminal(), P, "t1", "Containers")
    const state = setSessionEntrypoint(named, P, "t1", "lazydocker", false)
    const terminal = sessionsOf(state, P).find((s) => s.id === "t1")
    expect(terminal?.entrypoint).toBe("lazydocker")
    expect(terminal?.label).toBe("Containers")
  })

  it("clears the command without blanking the card's name", () => {
    const set = setSessionEntrypoint(withTerminal(), P, "t1", "lazygit", true)
    const state = setSessionEntrypoint(set, P, "t1", "", true)
    const terminal = sessionsOf(state, P).find((s) => s.id === "t1")
    expect(terminal?.entrypoint).toBe("")
    expect(terminal?.label).toBe("lazygit")
  })

  it("refuses a provider session, matching what the store will accept", () => {
    const state = withTerminal()
    expect(setSessionEntrypoint(state, P, "s1", "lazygit", true)).toBe(state)
  })

  it("does not mutate the input state", () => {
    const before = withTerminal()
    setSessionEntrypoint(before, P, "t1", "lazygit", true)
    expect(sessionsOf(before, P).find((s) => s.id === "t1")?.entrypoint).toBeUndefined()
  })

  it("ignores unknown project or session ids", () => {
    const state = withTerminal()
    expect(setSessionEntrypoint(state, "nope", "t1", "x", true)).toBe(state)
    expect(setSessionEntrypoint(state, P, "ghost", "x", true)).toBe(state)
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

describe("sidebarGroups", () => {
  const ids = (groups: ReturnType<typeof sidebarGroups>) =>
    groups.map((group) => [group.key, group.sessions.map((s) => s.id)])
  const wall = (id: string, cells: string[]) => ({ id, name: id, cells, cols: [], rows: [] })

  it("draws one block per worktree when nothing is pinned", () => {
    let state = addSession({}, P, "s1")
    state = addSession(state, P, "wt1", "claude", "/wt/a")
    expect(ids(sidebarGroups(sessionsOf(state, P)))).toEqual([
      [ROOT_GROUP_KEY, ["s1"]],
      ["/wt/a", ["wt1"]],
    ])
  })

  it("lifts the pinned sessions into a first block, keeping stored order", () => {
    let state = setSessionPinned(buildState(4), P, "s3", true)
    state = setSessionPinned(state, P, "s1", true)
    expect(ids(sidebarGroups(sessionsOf(state, P)))).toEqual([
      [PINNED_GROUP_KEY, ["s1", "s3"]],
      [ROOT_GROUP_KEY, ["s2", "s4"]],
    ])
  })

  // The block gathers cards from every checkout, so it is the one group whose
  // sessions do not share a path — and it leaves their worktree blocks behind.
  it("takes pinned sessions out of their worktree blocks", () => {
    let state = addSession({}, P, "s1")
    state = addSession(state, P, "a1", "claude", "/wt/a")
    state = addSession(state, P, "b1", "claude", "/wt/b")
    state = setSessionPinned(state, P, "a1", true)
    expect(ids(sidebarGroups(sessionsOf(state, P)))).toEqual([
      [PINNED_GROUP_KEY, ["a1"]],
      [ROOT_GROUP_KEY, ["s1"]],
      ["/wt/b", ["b1"]],
    ])
  })

  // The blocks answer the question a wall could not: which sessions are in it.
  // They are built from the groups, not from what is on screen, so they stand
  // whether or not any of those walls is the thing being drawn.
  it("lifts each wall into a block above everything, in the order it was arranged", () => {
    let state = addSession({}, P, "s1")
    state = addSession(state, P, "a1", "claude", "/wt/a")
    state = addSession(state, P, "b1", "claude", "/wt/b")
    expect(ids(sidebarGroups(sessionsOf(state, P), [wall("g1", ["b1", "s1"])]))).toEqual([
      ["g1", ["b1", "s1"]],
      ["/wt/a", ["a1"]],
    ])
  })

  // Several walls per project is the whole point: an orchestrator and the
  // worktrees it spawned are one, the next investigation and its own another.
  it("draws every wall, each as its own block", () => {
    const state = buildState(4)
    expect(
      ids(sidebarGroups(sessionsOf(state, P), [wall("g1", ["s1", "s2"]), wall("g2", ["s3"])])),
    ).toEqual([
      ["g1", ["s1", "s2"]],
      ["g2", ["s3"]],
      [ROOT_GROUP_KEY, ["s4"]],
    ])
  })

  it("draws the walls above the pinned block and takes their cards out of it", () => {
    let state = setSessionPinned(buildState(3), P, "s1", true)
    state = setSessionPinned(state, P, "s2", true)
    expect(ids(sidebarGroups(sessionsOf(state, P), [wall("g1", ["s2", "s3"])]))).toEqual([
      ["g1", ["s2", "s3"]],
      [PINNED_GROUP_KEY, ["s1"]],
    ])
  })

  it("draws no block for a wall with nothing live left in it", () => {
    const state = buildState(2)
    expect(ids(sidebarGroups(sessionsOf(state, P), [wall("g1", ["gone"])]))).toEqual([
      [ROOT_GROUP_KEY, ["s1", "s2"]],
    ])
  })

  // The block is display-only, so unpinning drops a session back among the
  // neighbours it was lifted over instead of stranding it on top.
  it("puts an unpinned session back where the stored order has it", () => {
    let state = setSessionPinned(buildState(3), P, "s3", true)
    expect(ids(sidebarGroups(sessionsOf(state, P)))).toEqual([
      [PINNED_GROUP_KEY, ["s3"]],
      [ROOT_GROUP_KEY, ["s1", "s2"]],
    ])
    state = setSessionPinned(state, P, "s3", false)
    expect(ids(sidebarGroups(sessionsOf(state, P)))).toEqual([[ROOT_GROUP_KEY, ["s1", "s2", "s3"]]])
  })

  it("marks only the pinned block, and gives it no path", () => {
    const state = setSessionPinned(buildState(2), P, "s2", true)
    const groups = sidebarGroups(sessionsOf(state, P))
    expect(groups.map((g) => [g.pinned, g.path])).toEqual([
      [true, ""],
      [false, ""],
    ])
  })

  it("has no blocks at all for a project with no sessions", () => {
    expect(sidebarGroups([])).toEqual([])
  })
})

describe("reorderSubset", () => {
  const s = (id: string, pinned?: boolean): Session => ({
    id,
    label: id,
    kind: "shell",
    ...(pinned ? { pinned } : {}),
  })
  const stored = [s("a"), s("p1", true), s("b"), s("c"), s("p2", true)]
  const unpinned = (session: Session) => !session.pinned

  it("reorders the subset and leaves every other session in place", () => {
    expect(reorderSubset(stored, ["c", "a", "b"], unpinned)).toEqual(["c", "p1", "a", "b", "p2"])
  })

  it("reorders the pinned block without moving the sessions between them", () => {
    expect(reorderSubset(stored, ["p2", "p1"], (x) => !!x.pinned)).toEqual([
      "a",
      "p2",
      "b",
      "c",
      "p1",
    ])
  })

  // A short list has to come back with a repeat, not a gap: reorderSessions
  // rejects an id set that does not match, which is what drops a stale drag.
  it("repeats rather than drops when the ids run out", () => {
    expect(reorderSubset(stored, ["c"], unpinned)).toEqual(["c", "p1", "b", "c", "p2"])
  })

  it("leaves the order alone when the predicate picks nothing", () => {
    expect(reorderSubset(stored, [], () => false)).toEqual(["a", "p1", "b", "c", "p2"])
  })
})

describe("neighborSessionId", () => {
  it("steps forward and backward through the list", () => {
    const state = buildState(3)
    expect(neighborSessionId(state, P, "s1", 1)).toBe("s2")
    expect(neighborSessionId(state, P, "s2", -1)).toBe("s1")
  })

  it("wraps at both ends", () => {
    const state = buildState(3)
    expect(neighborSessionId(state, P, "s3", 1)).toBe("s1")
    expect(neighborSessionId(state, P, "s1", -1)).toBe("s3")
  })

  // The sidebar draws the pinned block first, so the walk has to follow it — a
  // pinned session sits at the top of the list, not where it was created.
  it("walks the pinned order the sidebar shows", () => {
    const state = setSessionPinned(buildState(3), P, "s3", true) // s3, s1, s2
    expect(neighborSessionId(state, P, "s3", 1)).toBe("s1")
    expect(neighborSessionId(state, P, "s3", -1)).toBe("s2")
    expect(neighborSessionId(state, P, "s2", 1)).toBe("s3")
  })

  // A root session opened after a worktree one lands last in the stored list but
  // draws under its own group, so the walk follows the groups the sidebar shows.
  it("walks the worktree groups as they are drawn", () => {
    let state = addSession({}, P, "s1")
    state = addSession(state, P, "wt1", "claude", "/wt")
    state = addSession(state, P, "s2")
    expect(neighborSessionId(state, P, "s1", 1)).toBe("s2")
    expect(neighborSessionId(state, P, "s2", 1)).toBe("wt1")
    expect(neighborSessionId(state, P, "wt1", 1)).toBe("s1")
  })

  it("has nowhere to go with zero or one session", () => {
    expect(neighborSessionId({}, P, "", 1)).toBe("")
    expect(neighborSessionId(buildState(1), P, "s1", 1)).toBe("")
    expect(neighborSessionId(buildState(1), P, "s1", -1)).toBe("")
  })

  it("ignores an unknown project", () => {
    expect(neighborSessionId(buildState(3), "nope", "s1", 1)).toBe("")
  })

  // The active session can be gone (closed under a stale render), so the press
  // still lands on the end it steps in from instead of doing nothing.
  it("falls back to an end of the list for an unknown session", () => {
    const state = buildState(3)
    expect(neighborSessionId(state, P, "gone", 1)).toBe("s1")
    expect(neighborSessionId(state, P, "gone", -1)).toBe("s3")
  })
})

describe("setSessionPinned", () => {
  it("marks the session pinned without moving it", () => {
    const next = setSessionPinned(buildState(3), P, "s3", true)
    expect(sessionsOf(next, P).map((s) => s.id)).toEqual(["s1", "s2", "s3"])
    expect(sessionsOf(next, P)[2].pinned).toBe(true)
  })

  it("unpins a pinned session", () => {
    let state = setSessionPinned(buildState(3), P, "s3", true)
    state = setSessionPinned(state, P, "s3", false)
    expect(sessionsOf(state, P)[2].pinned).toBe(false)
  })

  it("leaves the active session and the label counter alone", () => {
    const state = buildState(3)
    const next = setSessionPinned(state, P, "s1", true)
    expect(activeSessionId(next, P)).toBe(activeSessionId(state, P))
    expect(next[P].nextSeq).toBe(state[P].nextSeq)
  })

  it("ignores unknown project and session ids", () => {
    const state = buildState(2)
    expect(setSessionPinned(state, "nope", "s1", true)).toBe(state)
    expect(setSessionPinned(state, P, "ghost", true)).toBe(state)
  })

  it("does not mutate the input state", () => {
    const state = buildState(3)
    setSessionPinned(state, P, "s3", true)
    expect(sessionsOf(state, P)[2].pinned).toBeUndefined()
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

  it("returns a codex session carrying a provider session id", () => {
    const state = withClaudeSession(buildState(2), "s1", "019fe876-0fb5", "codex")
    expect(resumableSession(state, P, "s1")).toMatchObject({
      id: "s1",
      providerSessionId: "019fe876-0fb5",
    })
  })

  it("returns an antigravity session carrying a provider session id", () => {
    const state = withClaudeSession(buildState(2), "s1", "7bb32ee5-e8e3", "antigravity")
    expect(resumableSession(state, P, "s1")).toMatchObject({
      id: "s1",
      providerSessionId: "7bb32ee5-e8e3",
    })
  })

  it("returns an omp session carrying a provider session id", () => {
    const state = withClaudeSession(buildState(2), "s1", "019ffb38-ceab", "omp")
    expect(resumableSession(state, P, "s1")).toMatchObject({
      id: "s1",
      providerSessionId: "019ffb38-ceab",
    })
  })

  it("returns an opencode session carrying a provider session id", () => {
    const state = withClaudeSession(buildState(2), "s1", "ses_0031a382dffe", "opencode")
    expect(resumableSession(state, P, "s1")).toMatchObject({
      id: "s1",
      providerSessionId: "ses_0031a382dffe",
    })
  })

  it("returns a crush session carrying a provider session id", () => {
    const state = withClaudeSession(buildState(2), "s1", "18345afc-f497", "crush")
    expect(resumableSession(state, P, "s1")).toMatchObject({
      id: "s1",
      providerSessionId: "18345afc-f497",
    })
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

// The undo behind the close toast: the same session comes back where it was,
// still carrying the conversation it ran, without minting a new numbered card.
describe("closeSession then restoreSession (undo)", () => {
  const closedAt = 1

  it("returns an equivalent session to its slot, conversation intact", () => {
    const before = withClaudeSession(buildState(3), "s2", "claude-abc")
    const closed = closeSession(before, P, "s2")
    const undone = restoreSession(closed, P, before[P].sessions[closedAt], closedAt)

    expect(sessionsOf(undone, P).map((s) => s.id)).toEqual(["s1", "s2", "s3"])
    expect(sessionsOf(undone, P)[closedAt]).toEqual(before[P].sessions[closedAt])
    expect(resumableSession(undone, P, "s2")?.providerSessionId).toBe("claude-abc")
    expect(activeSessionId(undone, P)).toBe("s2")
    expect(undone[P].nextSeq).toBe(before[P].nextSeq)
  })

  it("does nothing when the project was closed in between", () => {
    const before = buildState(3)
    const closed = removeProject(closeSession(before, P, "s2"), P)
    expect(restoreSession(closed, P, before[P].sessions[closedAt], closedAt)).toBe(closed)
  })
})

describe("activeTarget", () => {
  const root = "/repo"

  it("falls back to the project root when the active session has no checkout", () => {
    expect(activeTarget(buildState(2), P, root)).toEqual({
      sessionId: "s2",
      path: root,
      kind: "claude",
      sandboxed: false,
    })
  })

  it("resolves a worktree session to its checkout, and reports what it runs", () => {
    const state = restoreSession(buildState(1), P, {
      id: "wt1",
      label: "swift-rabbit",
      kind: "shell",
      path: "/repo/.worktrees/swift-rabbit",
    })
    expect(activeTarget(state, P, root)).toEqual({
      sessionId: "wt1",
      path: "/repo/.worktrees/swift-rabbit",
      kind: "shell",
      sandboxed: false,
    })
  })

  // The footer's attach button reads this: a confined session is handed a copy
  // of anything outside its checkout, and it has to know which sessions those
  // are before the picker opens.
  it("reports a confined session as sandboxed", () => {
    const state = restoreSession(buildState(1), P, {
      id: "wt1",
      label: "swift-rabbit",
      kind: "claude",
      path: "/repo/.worktrees/swift-rabbit",
      sandboxed: true,
    })
    expect(activeTarget(state, P, root).sandboxed).toBe(true)
  })

  it("keeps the project root when there is no session at all", () => {
    expect(activeTarget({}, P, root)).toEqual({
      sessionId: "",
      path: root,
      kind: "",
      sandboxed: false,
    })
  })

  it("is empty off a project route", () => {
    expect(activeTarget(buildState(2), null, "")).toEqual({
      sessionId: "",
      path: "",
      kind: "",
      sandboxed: false,
    })
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

describe("groupByWorktree", () => {
  const s = (id: string, path?: string): Session => ({
    id,
    label: id,
    kind: "shell",
    ...(path ? { path } : {}),
  })

  it("returns no groups for an empty list", () => {
    expect(groupByWorktree([])).toEqual([])
  })

  it("keeps pathless sessions in the root group keyed by ''", () => {
    const groups = groupByWorktree([s("s1"), s("s2")])
    expect(groups).toHaveLength(1)
    expect(groups[0].path).toBe("")
    expect(groups[0].sessions.map((x) => x.id)).toEqual(["s1", "s2"])
  })

  it("buckets by checkout path in first-appearance order", () => {
    const groups = groupByWorktree([s("s1"), s("wt1", "/wt/a"), s("wt2", "/wt/b")])
    expect(groups.map((g) => g.path)).toEqual(["", "/wt/a", "/wt/b"])
  })

  it("merges interleaved paths into one group, preserving flat order", () => {
    const groups = groupByWorktree([s("a1", "/wt/a"), s("b1", "/wt/b"), s("a2", "/wt/a")])
    expect(groups.map((g) => g.path)).toEqual(["/wt/a", "/wt/b"])
    expect(groups[0].sessions.map((x) => x.id)).toEqual(["a1", "a2"])
    expect(groups[1].sessions.map((x) => x.id)).toEqual(["b1"])
  })
})

describe("orderGroups", () => {
  const s = (id: string, path?: string): Session => ({
    id,
    label: id,
    kind: "shell",
    ...(path ? { path } : {}),
  })
  const groups = sidebarGroups([
    s("r1"),
    s("r2"),
    s("a1", "/wt/a"),
    s("b1", "/wt/b"),
    s("b2", "/wt/b"),
  ])

  it("moves a group's whole block, keeping its internal order", () => {
    const ids = orderGroups(groups, ["/wt/b", ROOT_GROUP_KEY, "/wt/a"])
    expect(ids).toEqual(["b1", "b2", "r1", "r2", "a1"])
  })

  it("keys the pathless root group by ROOT_GROUP_KEY, not by ''", () => {
    expect(orderGroups(groups, [ROOT_GROUP_KEY])).toEqual(["r1", "r2"])
    expect(orderGroups(groups, [""])).toEqual([])
  })

  // A short list is what reorderSessions rejects, so a group closed mid-drag
  // drops the whole stale order instead of taking its sessions with it.
  it("contributes nothing for a key naming no group", () => {
    expect(orderGroups(groups, ["/wt/gone"])).toEqual([])
  })
})

describe("adoptSession", () => {
  const opened: Session = { id: "agent-1", label: "auth-fix", kind: "claude", path: "/wt/auth-fix" }

  it("appends a session the backend already created", () => {
    const state = adoptSession(buildState(2), P, opened, 4)
    expect(sessionsOf(state, P).map((s) => s.id)).toEqual(["s1", "s2", "agent-1"])
    expect(state[P].nextSeq).toBe(4)
  })

  it("leaves focus where the user left it", () => {
    const before = buildState(2)
    const state = adoptSession(before, P, opened, 4)
    expect(activeSessionId(state, P)).toBe(activeSessionId(before, P))
  })

  it("ignores a session that is already there", () => {
    const once = adoptSession(buildState(2), P, opened, 4)
    expect(adoptSession(once, P, opened, 4)).toBe(once)
  })

  it("ignores a project the window does not hold", () => {
    const before = buildState(2)
    expect(adoptSession(before, "other", opened, 4)).toBe(before)
  })

  // The row was written with the backend's counter; a page that had already
  // moved past it must not walk the number back and mint a duplicate label.
  it("never lowers the label counter", () => {
    const before = buildState(5)
    const state = adoptSession(before, P, opened, 2)
    expect(state[P].nextSeq).toBe(before[P].nextSeq)
  })
})

describe("sessionOrigin", () => {
  // The child of s1, as the backend hands it over: the id of the session that
  // asked for it, plus the name that session went by at the time.
  const child: Session = {
    id: "worker",
    label: "auth-fix",
    kind: "claude",
    originSessionId: "s1",
    originLabel: "Session 1",
  }

  it("names nothing for a session nobody delegated", () => {
    const state = buildState(2)
    expect(sessionOrigin(state, state[P].sessions[0])).toBe("")
  })

  // The whole reason the id is stored beside the label: a renamed parent is
  // still the parent, and the card has to say what it is called now.
  it("follows a rename of the parent", () => {
    const state = renameSession(adoptSession(buildState(2), P, child, 3), P, "s1", "planner")
    expect(sessionOrigin(state, child)).toBe("planner")
  })

  // And the whole reason the label is stored beside the id: the delegation
  // happened, and a closed parent does not make that untrue.
  it("falls back to the name the parent had when it is gone", () => {
    const state = closeSession(adoptSession(buildState(2), P, child, 3), P, "s1")
    expect(sessionOrigin(state, child)).toBe("Session 1")
  })

  // Delegation crosses projects, so the lookup has to: the sidebar shows one
  // project at a time and the parent can be in any of them.
  it("finds a parent in another project", () => {
    const state = adoptSession(addSession(buildState(2), "other", "o1"), "other", child, 2)
    expect(sessionOrigin(state, state.other.sessions[1])).toBe("Session 1")
  })

  // An origin whose parent is gone and that never carried a name draws nothing
  // rather than an empty "from".
  it("names nothing when neither half resolves", () => {
    expect(
      sessionOrigin(buildState(1), { ...child, originSessionId: "ghost", originLabel: "" }),
    ).toBe("")
  })
})

describe("hasSession", () => {
  it("finds a session under any project", () => {
    let state = buildState(2)
    state = addSession(state, "other", "x1")
    expect(hasSession(state, "s1")).toBe(true)
    expect(hasSession(state, "x1")).toBe(true)
  })

  // The question a terminal asks as it goes away: a session that left the
  // workspace is one whose PTY may die, and a session still in it is a card
  // React took down for reasons of its own.
  it("is false for a session that left the workspace", () => {
    const state = closeSession(buildState(2), P, "s2")
    expect(hasSession(state, "s2")).toBe(false)
    expect(hasSession(state, "never-existed")).toBe(false)
    expect(hasSession({}, "s1")).toBe(false)
  })
})

describe("dropClosedSession", () => {
  it("removes the card and takes the active session the row recorded", () => {
    const state = dropClosedSession(buildState(3), P, "s2", "s3")
    expect(sessionsOf(state, P).map((s) => s.id)).toEqual(["s1", "s3"])
    expect(activeSessionId(state, P)).toBe("s3")
  })

  // The backend picked the neighbour when it wrote the row; picking a different
  // one here would put the window and the next launch on different sessions.
  it("takes an empty active session as the project having none left", () => {
    const state = dropClosedSession(addSession({}, P, "only"), P, "only", "")
    expect(sessionsOf(state, P)).toEqual([])
    expect(activeSessionId(state, P)).toBe("")
  })

  it("ignores a session or project it does not hold", () => {
    const before = buildState(2)
    expect(dropClosedSession(before, P, "ghost", "s1")).toBe(before)
    expect(dropClosedSession(before, "other", "s1", "")).toBe(before)
  })
})

describe("setSessionSandboxed", () => {
  const state = () => buildState(2)

  it("marks the session the spawn reported", () => {
    const next = setSessionSandboxed(state(), "s1", true)
    expect(next[P]?.sessions[0]?.sandboxed).toBe(true)
    expect(next[P]?.sessions[1]?.sandboxed).toBeUndefined()
  })

  it("clears the mark when a respawn reports it unconfined", () => {
    const confined = setSessionSandboxed(state(), "s1", true)
    const next = setSessionSandboxed(confined, "s1", false)
    expect(next[P]?.sessions[0]?.sandboxed).toBeUndefined()
  })

  // The event fires on every spawn, so an unchanged answer has to return the
  // same object: a new one re-renders every card in the project for nothing.
  it("returns the same state when nothing changed", () => {
    const current = state()
    expect(setSessionSandboxed(current, "s1", false)).toBe(current)
    const confined = setSessionSandboxed(current, "s1", true)
    expect(setSessionSandboxed(confined, "s1", true)).toBe(confined)
  })

  it("ignores a session it does not know", () => {
    const current = state()
    expect(setSessionSandboxed(current, "gone", true)).toBe(current)
  })
})

// The mark is dropped rather than set to undefined: the two hydration paths omit
// the key entirely, and a session that reached the same state two ways must not
// carry two different shapes.
describe("setSessionSandboxed shape", () => {
  it("leaves no sandboxed key on an unconfined session", () => {
    const confined = setSessionSandboxed(buildState(1), "s1", true)
    const cleared = setSessionSandboxed(confined, "s1", false)
    const session = cleared[P]?.sessions[0]
    expect(session && "sandboxed" in session).toBe(false)
  })
})
