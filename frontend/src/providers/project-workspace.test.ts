import { describe, expect, it } from "vitest"
import type { StoredProject, StoredSession } from "@/lib/api-types"
import { buildSessionState, toProject } from "./project-workspace"

const storedSession = (overrides: Partial<StoredSession> = {}): StoredSession => ({
  id: "s1",
  label: "Session 1",
  kind: "claude",
  path: "",
  providerSessionId: "",
  entrypoint: "",
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
