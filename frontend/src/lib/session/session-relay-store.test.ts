import { describe, expect, it } from "vitest"
import { isRelayStalledEvent } from "./session-events"
import { createSessionRelayStore } from "./session-relay-store"

describe("isRelayStalledEvent", () => {
  it("accepts a request whose target holds the answer", () => {
    expect(isRelayStalledEvent({ id: "s1", targetId: "s2", target: "docs" })).toBe(true)
  })

  it("accepts one with no sender — the command line has no card", () => {
    expect(isRelayStalledEvent({ id: "", targetId: "s2", target: "docs" })).toBe(true)
  })

  it("refuses one with no target, which is the only thing the toast can open", () => {
    expect(isRelayStalledEvent({ id: "s1", targetId: "", target: "docs" })).toBe(false)
    expect(isRelayStalledEvent({ id: "s1", target: "docs" })).toBe(false)
    expect(isRelayStalledEvent(null)).toBe(false)
  })
})

// A hand-driven event source: the test emits, the store reacts.
function source() {
  const handlers: Array<(data: unknown) => void> = []
  return {
    subscribe: (handler: (data: unknown) => void) => {
      handlers.push(handler)
      return () => {}
    },
    emit: (data: unknown) => {
      for (const handler of handlers) {
        handler(data)
      }
    },
  }
}

function build() {
  const relay = source()
  const status = source()
  const store = createSessionRelayStore(relay.subscribe, status.subscribe)
  return { relay, status, store }
}

describe("createSessionRelayStore", () => {
  it("is empty until something is reported", () => {
    const { store } = build()
    expect(store.get("s1")).toBeNull()
  })

  it("marks both ends of one request independently", () => {
    const { relay, store } = build()

    relay.emit({ id: "s1", peer: "docs", direction: "out", ticket: "fd94999a" })
    relay.emit({ id: "s2", peer: "auth", direction: "in", ticket: "a1b2c3d4" })

    expect(store.get("s1")).toEqual({ peer: "docs", direction: "out", ticket: "fd94999a" })
    expect(store.get("s2")).toEqual({ peer: "auth", direction: "in", ticket: "a1b2c3d4" })
  })

  it("clears on the empty direction the backend sends when the errand closes", () => {
    const { relay, store } = build()

    relay.emit({ id: "s1", peer: "docs", direction: "out" })
    relay.emit({ id: "s1", peer: "", direction: "" })

    expect(store.get("s1")).toBeNull()
  })

  it("keeps a request with no peer label — the other end is the command line", () => {
    const { relay, store } = build()

    relay.emit({ id: "s2", peer: "", direction: "in", ticket: "fd94999a" })

    expect(store.get("s2")).toEqual({ peer: "", direction: "in", ticket: "fd94999a" })
  })

  it("shows nothing for a direction this build cannot draw", () => {
    const { relay, store } = build()

    relay.emit({ id: "s1", peer: "docs", direction: "sideways" })

    expect(store.get("s1")).toBeNull()
  })

  it("ignores a payload with no session id", () => {
    const { relay, store } = build()

    relay.emit({ peer: "docs", direction: "out" })
    relay.emit(null)
    relay.emit("nonsense")

    expect(store.get("s1")).toBeNull()
  })

  it("clears when the session ends, which the relay's own clear would not reach", () => {
    const { relay, status, store } = build()

    relay.emit({ id: "s2", peer: "auth", direction: "in" })
    status.emit({ id: "s2", state: "idle" })

    expect(store.get("s2")).toBeNull()
  })

  it("survives every other state: a busy session is still owed an answer", () => {
    const { relay, status, store } = build()

    relay.emit({ id: "s2", peer: "auth", direction: "in", ticket: "fd94999a" })
    for (const state of ["busy", "done", "waiting"]) {
      status.emit({ id: "s2", state })
    }

    expect(store.get("s2")).toEqual({ peer: "auth", direction: "in", ticket: "fd94999a" })
  })

  it("keeps a request from a backend that sends no ticket", () => {
    const { relay, store } = build()

    relay.emit({ id: "s2", peer: "auth", direction: "in" })

    expect(store.get("s2")).toEqual({ peer: "auth", direction: "in", ticket: "" })
  })

  it("notifies when only the ticket changed — it is the errand, not a detail", () => {
    const { relay, store } = build()
    let notifications = 0
    store.subscribe("s2", () => {
      notifications += 1
    })

    relay.emit({ id: "s2", peer: "auth", direction: "in", ticket: "fd94999a" })
    relay.emit({ id: "s2", peer: "auth", direction: "in", ticket: "a1b2c3d4" })

    expect(notifications).toBe(2)
    expect(store.get("s2")).toEqual({ peer: "auth", direction: "in", ticket: "a1b2c3d4" })
  })

  it("does not notify subscribers when a repeat report changes nothing", () => {
    const { relay, store } = build()
    let notifications = 0
    store.subscribe("s1", () => {
      notifications += 1
    })

    relay.emit({ id: "s1", peer: "docs", direction: "out", ticket: "fd94999a" })
    relay.emit({ id: "s1", peer: "docs", direction: "out", ticket: "fd94999a" })

    expect(notifications).toBe(1)
  })
})
