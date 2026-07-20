import {describe, expect, it} from "vitest"
import {createSessionAgentStore} from "./session-agent-store"

// harness wires a store to hand-driven agent and status sources.
function harness() {
  let onAgent: (data: unknown) => void = () => {}
  let onStatus: (data: unknown) => void = () => {}
  const store = createSessionAgentStore(
    (h) => {
      onAgent = h
      return () => {}
    },
    (h) => {
      onStatus = h
      return () => {}
    },
  )
  return {
    store,
    agent: (data: unknown) => onAgent(data),
    status: (data: unknown) => onStatus(data),
  }
}

describe("createSessionAgentStore", () => {
  it("returns null while nothing has been reported", () => {
    const {store} = harness()
    expect(store.get("s1")).toBeNull()
  })

  it("keeps the reported agent per session", () => {
    const {store, agent} = harness()
    agent({id: "s1", agent: "claude"})
    expect(store.get("s1")).toBe("claude")
    expect(store.get("s2")).toBeNull()
  })

  it("clears the mark on an empty agent, like a PTY respawn", () => {
    const {store, agent} = harness()
    agent({id: "s1", agent: "claude"})
    agent({id: "s1", agent: ""})
    expect(store.get("s1")).toBeNull()
  })

  it("clears the mark when the session reports idle (SessionEnd)", () => {
    const {store, agent, status} = harness()
    agent({id: "s1", agent: "claude"})
    status({id: "s1", state: "idle"})
    expect(store.get("s1")).toBeNull()
  })

  it("keeps the mark through live states", () => {
    const {store, agent, status} = harness()
    agent({id: "s1", agent: "claude"})
    for (const state of ["busy", "waiting", "done"]) {
      status({id: "s1", state})
    }
    expect(store.get("s1")).toBe("claude")
  })

  it("drops an agent no known kind names", () => {
    const {store, agent} = harness()
    agent({id: "s1", agent: "claude"})
    agent({id: "s1", agent: "future-provider"})
    expect(store.get("s1")).toBeNull()
  })

  it("notifies subscribers only on change", () => {
    const {store, agent} = harness()
    let calls = 0
    store.subscribe("s1", () => calls++)
    agent({id: "s1", agent: "claude"})
    agent({id: "s1", agent: "claude"})
    expect(calls).toBe(1)
  })

  it("retains the mark across an unsubscribe, like a card unmount", () => {
    const {store, agent} = harness()
    const off = store.subscribe("s1", () => {})
    agent({id: "s1", agent: "claude"})
    off()
    expect(store.get("s1")).toBe("claude")
  })

  it("ignores malformed payloads on both sources", () => {
    const {store, agent, status} = harness()
    agent({id: "s1"})
    agent(null)
    status({id: "s1"})
    expect(store.get("s1")).toBeNull()
  })
})
