// @vitest-environment jsdom
//
// The harness has to be imported before anything that reaches react-dom (see
// @/test/render-budget).
import { mountBudget } from "@/test/render-budget"
import { createElement } from "react"
import type { NavigateFunction } from "react-router-dom"
import { describe, expect, it, vi } from "vitest"
import type { Project } from "@/lib/api-types"
import { FILED_EVENT } from "@/lib/session/session-events"
import type { SessionState } from "@/lib/session/sessions"
import { useSessionEvents } from "./project-events"

// A stand-in for the backend's event stream: the real module opens a WebSocket
// and reconnects forever when it cannot.
const bus = vi.hoisted(() => {
  const handlers = new Map<string, Set<(data: unknown) => void>>()
  return {
    on(name: string, callback: (data: unknown) => void) {
      let set = handlers.get(name)
      if (!set) {
        set = new Set()
        handlers.set(name, set)
      }
      set.add(callback)
      return () => {
        handlers.get(name)?.delete(callback)
      }
    },
    emit(name: string, data: unknown) {
      for (const callback of handlers.get(name) ?? []) {
        callback(data)
      }
    },
  }
})

vi.mock("@/lib/app-events", () => ({ onAppEvent: bus.on, dispatchEnvelope: () => {} }))

async function mountWith(state: SessionState) {
  const sessionsRef = { current: state }
  const commit = vi.fn((next: SessionState) => {
    sessionsRef.current = next
  })
  function Probe() {
    useSessionEvents({
      sessions: state,
      activeProjectId: "p1",
      sessionsRef,
      projectsRef: { current: [] as Project[] },
      activeProjectIdRef: { current: "p1" },
      desktopNotificationsRef: { current: null },
      finishedTurnNotificationsRef: { current: false },
      lastStatusRef: { current: new Map() },
      commit,
      setProjects: vi.fn(),
      setAskNotifications: vi.fn(),
      activateSession: vi.fn(),
      navigate: vi.fn() as unknown as NavigateFunction,
    })
    return null
  }
  await mountBudget(createElement(Probe))
  return { sessionsRef, commit }
}

const workspace: SessionState = {
  p1: {
    activeId: "s1",
    nextSeq: 4,
    sessions: [
      { id: "s1", label: "planner", kind: "claude" },
      { id: "s2", label: "auth-fix", kind: "claude", folder: "Apps" },
      { id: "s3", label: "infra", kind: "claude", folder: "Apps" },
    ],
  },
}

// `lich file` and `lich rename-folder` already wrote the rows; the window only
// moves the cards the event names.
describe("sessions-filed", () => {
  it("moves the named cards into the folder", async () => {
    const { sessionsRef, commit } = await mountWith(workspace)

    bus.emit(FILED_EVENT, { projectId: "p1", ids: ["s1", "s2"], folder: "Infra" })

    expect(commit).toHaveBeenCalledTimes(1)
    const folders = sessionsRef.current.p1.sessions.map((s) => s.folder)
    expect(folders).toEqual(["Infra", "Infra", "Apps"])
  })

  it("takes the cards out when the folder is empty", async () => {
    const { sessionsRef } = await mountWith(workspace)

    bus.emit(FILED_EVENT, { projectId: "p1", ids: ["s2", "s3"], folder: "" })

    expect(sessionsRef.current.p1.sessions.every((s) => s.folder === undefined)).toBe(true)
  })

  it("commits nothing for a payload it cannot read or cards it does not have", async () => {
    const { commit } = await mountWith(workspace)

    bus.emit(FILED_EVENT, { ids: ["s1"], folder: "Apps" })
    bus.emit(FILED_EVENT, { projectId: "p1", ids: ["parked"], folder: "Apps" })
    bus.emit(FILED_EVENT, { projectId: "p9", ids: ["s1"], folder: "Apps" })

    expect(commit).not.toHaveBeenCalled()
  })
})
