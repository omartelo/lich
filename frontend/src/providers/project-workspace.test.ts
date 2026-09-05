import { describe, expect, it } from "vitest"
import type { StoredProject, StoredSession } from "@/lib/api-types"
import { toOpenedProject } from "@/lib/session/session-events"
import { adoptSession } from "@/lib/session/sessions"
import { buildSessionState, toProject } from "./project-workspace"

const storedSession = (overrides: Partial<StoredSession> = {}): StoredSession => ({
  id: "s1",
  label: "Session 1",
  kind: "claude",
  path: "",
  providerSessionId: "",
  entrypoint: "",
  sandbox: "",
  pinned: false,
  originSessionId: "",
  originLabel: "",
  ...overrides,
})

const storedProject = (overrides: Partial<StoredProject> = {}): StoredProject => ({
  id: "p1",
  name: "alpha",
  path: "/tmp/alpha",
  nextSeq: 2,
  activeSessionId: "",
  defaultProvider: "",
  sessions: [storedSession()],
  ...overrides,
})

describe("toProject", () => {
  it("keeps only the fields a tab needs", () => {
    expect(toProject(storedProject({ defaultProvider: "codex", nextSeq: 9 }))).toEqual({
      id: "p1",
      name: "alpha",
      path: "/tmp/alpha",
    })
  })
})

describe("buildSessionState", () => {
  it("returns an empty state for no projects", () => {
    expect(buildSessionState([])).toEqual({})
  })

  it("reads a null session list as a project with no sessions", () => {
    const state = buildSessionState([storedProject({ sessions: null })])
    expect(state.p1).toEqual({ sessions: [], activeId: "", nextSeq: 2 })
  })

  it("restores a terminal's entrypoint, and leaves the field off a plain shell", () => {
    const state = buildSessionState([
      storedProject({
        sessions: [
          storedSession({ id: "t1", kind: "shell", entrypoint: "lazygit" }),
          storedSession({ id: "t2", kind: "shell" }),
        ],
      }),
    ])
    expect(state.p1.sessions[0].entrypoint).toBe("lazygit")
    expect(state.p1.sessions[1]).not.toHaveProperty("entrypoint")
  })

  it("falls back to Claude for a kind this build does not know", () => {
    const state = buildSessionState([
      storedProject({ sessions: [storedSession({ kind: "some-future-agent" })] }),
    ])
    expect(state.p1.sessions[0].kind).toBe("claude")
  })

  it("focuses the first session when no active one was stored", () => {
    const state = buildSessionState([
      storedProject({
        activeSessionId: "",
        sessions: [storedSession({ id: "s1" }), storedSession({ id: "s2" })],
      }),
    ])
    expect(state.p1.activeId).toBe("s1")
  })

  it("keeps the stored active session over the first one", () => {
    const state = buildSessionState([
      storedProject({
        activeSessionId: "s2",
        sessions: [storedSession({ id: "s1" }), storedSession({ id: "s2" })],
      }),
    ])
    expect(state.p1.activeId).toBe("s2")
  })

  it("drops the optional fields a row left empty", () => {
    const state = buildSessionState([storedProject()])
    expect(state.p1.sessions[0]).toEqual({ id: "s1", label: "Session 1", kind: "claude" })
  })

  it("carries the optional fields a row does set", () => {
    const state = buildSessionState([
      storedProject({
        sessions: [
          storedSession({
            path: "/tmp/alpha-wt",
            providerSessionId: "conv-1",
            pinned: true,
            originSessionId: "s0",
            originLabel: "Parent",
          }),
        ],
      }),
    ])
    expect(state.p1.sessions[0]).toEqual({
      id: "s1",
      label: "Session 1",
      kind: "claude",
      path: "/tmp/alpha-wt",
      providerSessionId: "conv-1",
      pinned: true,
      originSessionId: "s0",
      originLabel: "Parent",
    })
  })

  it("keys every project separately", () => {
    const state = buildSessionState([
      storedProject(),
      storedProject({ id: "p2", sessions: [storedSession({ id: "s9" })], nextSeq: 5 }),
    ])
    expect(Object.keys(state)).toEqual(["p1", "p2"])
    expect(state.p2).toMatchObject({ activeId: "s9", nextSeq: 5 })
  })
})

describe("confined sessions", () => {
  it("marks a session the spawn recorded as confined", () => {
    const state = buildSessionState([
      storedProject({ sessions: [storedSession({ id: "s1", sandbox: "on" })] }),
    ])
    expect(state.p1?.sessions[0]?.sandboxed).toBe(true)
  })

  // "off" and "" are both "not confined" — the card carries no mark either way,
  // and the difference (nobody decided yet) belongs to the spawn, not here.
  it("leaves every other value unmarked", () => {
    for (const sandbox of ["", "off", "maybe"]) {
      const state = buildSessionState([
        storedProject({ sessions: [storedSession({ id: "s1", sandbox })] }),
      ])
      expect(state.p1?.sessions[0]?.sandboxed).toBeUndefined()
    }
  })
})

// The two halves of an agent opening a project it had to put on screen first:
// the tab arrives as a payload rather than a reload, and the session event right
// behind it has to find the project already there — adoptSession drops a card
// whose project it does not know.
describe("a project opened outside the window", () => {
  const payload = (): unknown => ({
    id: "p9",
    name: "revu",
    path: "/src/revu",
    nextSeq: 7,
    activeSessionId: "parked",
    defaultProvider: "",
    sessions: [storedSession({ id: "parked", label: "Session 6" })],
  })

  it("takes the tab in with the sessions it was closed with", () => {
    const project = toOpenedProject(payload())
    expect(project).not.toBeNull()
    const state = buildSessionState([project as StoredProject])
    expect(state.p9).toMatchObject({ activeId: "parked", nextSeq: 7 })
    expect(state.p9?.sessions).toHaveLength(1)
  })

  it("holds the session that follows it", () => {
    const project = toOpenedProject(payload()) as StoredProject
    const state = { ...buildSessionState([project]) }
    const next = adoptSession(state, "p9", { id: "new", label: "Session 7", kind: "claude" }, 8)
    expect(next.p9?.sessions.map((s) => s.id)).toEqual(["parked", "new"])
    expect(next.p9?.nextSeq).toBe(8)
  })

  it("refuses a payload with no project in it", () => {
    expect(toOpenedProject({ id: "", name: "revu", path: "/src/revu" })).toBeNull()
    expect(toOpenedProject({ id: "p9", name: "", path: "/src/revu" })).toBeNull()
    expect(toOpenedProject({ id: "p9", name: "revu", path: "" })).toBeNull()
  })

  // A brand-new project has no sessions, and they cross as null rather than []
  // — a nil slice in Go.
  it("reads a project with no sessions", () => {
    const project = toOpenedProject({
      ...(payload() as object),
      sessions: null,
      activeSessionId: "",
      nextSeq: 1,
    })
    expect(buildSessionState([project as StoredProject]).p9).toMatchObject({
      sessions: [],
      activeId: "",
      nextSeq: 1,
    })
  })
})
