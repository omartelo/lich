// @vitest-environment jsdom
//
// The restored-session default, from the stored choice to the resume id the
// terminal spawns with. spawn-gate.test.ts pins the decision; this pins that
// TerminalHost acts on it.
//
// The harness is imported first for the reason render-budget.test.tsx names.
import { mountBudget } from "@/test/render-budget"
import { TerminalHost } from "./TerminalHost"
import { createElement } from "react"
import { HashRouter } from "react-router-dom"
import { expect, test, vi } from "vitest"
import { ProjectsContext, type ProjectsValue } from "@/providers/projects-context"

const backend = vi.hoisted(() => ({ onRestore: "" }))
const never = () => new Promise<never>(() => {})

vi.mock("@/lib/app-events", () => ({ onAppEvent: () => () => {}, dispatchEnvelope: () => {} }))
vi.mock("@/lib/rpc", () => {
  const hangs = new Proxy({}, { get: () => never })
  return {
    endpoint: () => ({ base: "http://localhost", token: "token" }),
    Terminal: new Proxy(
      {},
      {
        get: (_target, name) => {
          if (name === "WorkdirMissing") return async () => false
          if (name === "ResumeAvailable") return async () => true
          return never
        },
      },
    ),
    Store: new Proxy(
      {},
      { get: (_target, name) => (name === "GetSetting" ? async () => backend.onRestore : never) },
    ),
    ProjectService: hangs,
    Providers: hangs,
    System: hangs,
  }
})

const spawnedWith = vi.hoisted(() => ({ resume: {} as Record<string, string> }))
vi.mock("@/components/TerminalView", () => ({
  TerminalView: ({ sessionId, resume }: { sessionId: string; resume: string }) => {
    spawnedWith.resume[sessionId] = resume
    return null
  },
}))

const noop = () => {}

function workspace(sessionId: string): ProjectsValue {
  return {
    projects: [{ id: "p1", name: "repo", path: "/repo" }],
    sessions: {
      p1: {
        sessions: [{ id: sessionId, label: "docs", kind: "claude", providerSessionId: "conv-1" }],
        activeId: sessionId,
        nextSeq: 2,
      },
    },
    homeId: null,
    openProject: async () => {},
    openRecent: async () => true,
    ensureHomeProject: async () => "p1",
    closeProject: noop,
    newSession: () => "",
    newWorktreeSession: () => "",
    reopenWorktreeSession: async () => "",
    resumeClosedSession: async () => {},
    closeSession: noop,
    discardSession: noop,
    keepSession: noop,
    activateSession: noop,
    renameSession: noop,
    setEntrypoint: noop,
    scheduleSession: noop,
    pinSession: noop,
    fileSessions: noop,
    renameSessionFolder: noop,
    reorderProjects: noop,
    reorderSessions: noop,
  }
}

// Each test takes its own session id: the terminal registry remembers what has
// spawned, and a spawned id never goes back through the gate.
async function mountHost(sessionId: string) {
  window.location.hash = "#/projects/p1"
  const mounted = await mountBudget(
    createElement(
      HashRouter,
      null,
      createElement(
        ProjectsContext.Provider,
        { value: workspace(sessionId) },
        createElement(TerminalHost, null),
      ),
    ),
  )
  await mounted.act(() => {})
  await mounted.act(() => {})
  return mounted
}

test("a provider set to resume spawns the card on its conversation, unasked", async () => {
  backend.onRestore = "resume"
  const mounted = await mountHost("resume-card")
  expect(spawnedWith.resume["resume-card"]).toBe("conv-1")
  expect(document.body.textContent).not.toContain("Resume previous session?")
  await mounted.unmount()
})

test("a provider set to start new spawns the card empty, unasked", async () => {
  backend.onRestore = "fresh"
  const mounted = await mountHost("fresh-card")
  expect(spawnedWith.resume["fresh-card"]).toBe("")
  expect(document.body.textContent).not.toContain("Resume previous session?")
  await mounted.unmount()
})

test("a provider left on ask holds the spawn behind the prompt", async () => {
  backend.onRestore = ""
  const mounted = await mountHost("ask-card")
  expect(spawnedWith.resume["ask-card"]).toBeUndefined()
  expect(document.body.textContent).toContain("Resume previous session?")
  await mounted.unmount()
})
