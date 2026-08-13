import { describe, expect, it } from "vitest"
import { toInboxCount } from "./session-events"
import { createSessionInboxStore } from "./session-inbox-store"

describe("toInboxCount", () => {
  it("reads a positive count", () => {
    expect(toInboxCount({ id: "s1", count: 2 })).toBe(2)
  })

  it("reads anything else as zero, which clears the mark", () => {
    expect(toInboxCount({ id: "s1", count: 0 })).toBe(0)
    expect(toInboxCount({ id: "s1", count: -1 })).toBe(0)
    expect(toInboxCount({ id: "s1", count: "2" })).toBe(0)
    expect(toInboxCount({ id: "s1" })).toBe(0)
    expect(toInboxCount(null)).toBe(0)
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
  const inbox = source()
  const status = source()
  const store = createSessionInboxStore(inbox.subscribe, status.subscribe)
  return { inbox, status, store }
}

describe("createSessionInboxStore", () => {
  it("is zero until something is reported", () => {
    const { store } = build()
    expect(store.get("s1")).toBe(0)
  })

  it("follows the count up and back to zero", () => {
    const { inbox, store } = build()

    inbox.emit({ id: "s1", count: 2 })
    expect(store.get("s1")).toBe(2)

    inbox.emit({ id: "s1", count: 0 })
    expect(store.get("s1")).toBe(0)
  })

  it("keeps sessions apart", () => {
    const { inbox, store } = build()

    inbox.emit({ id: "s1", count: 1 })
    expect(store.get("s2")).toBe(0)
  })

  it("clears when the session ends — nothing there collects anymore", () => {
    const { inbox, status, store } = build()

    inbox.emit({ id: "s1", count: 3 })
    status.emit({ id: "s1", state: "idle" })

    expect(store.get("s1")).toBe(0)
  })
})
