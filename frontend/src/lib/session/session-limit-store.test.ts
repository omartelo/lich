import { describe, expect, it } from "vitest"
import { isScheduleEvent, toSessionLimit } from "./session-events"
import { createSessionLimitStore } from "./session-limit-store"

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
  const limit = source()
  const status = source()
  const store = createSessionLimitStore(limit.subscribe, status.subscribe)
  return { limit, status, store }
}

describe("toSessionLimit", () => {
  it("reads the window and the reset", () => {
    expect(toSessionLimit({ id: "s1", window: "weekly", resetsAt: 1700 })).toEqual({
      window: "weekly",
      resetsAt: 1700,
    })
  })

  it("keeps a limit whose window this build has no name for", () => {
    expect(toSessionLimit({ id: "s1", window: "monthly", resetsAt: 1700 })).toEqual({
      window: "",
      resetsAt: 1700,
    })
  })

  it("refuses a payload with no reset", () => {
    expect(toSessionLimit({ id: "s1", window: "session" })).toBeNull()
    expect(toSessionLimit(null)).toBeNull()
  })
})

describe("isScheduleEvent", () => {
  it("accepts the delivery's clear and lich's own park", () => {
    expect(isScheduleEvent({ id: "s1", at: 0 })).toBe(true)
    expect(isScheduleEvent({ id: "s1", at: 1700, prompt: "go on" })).toBe(true)
  })

  it("refuses a prompt that is not text", () => {
    expect(isScheduleEvent({ id: "s1", at: 1700, prompt: 3 })).toBe(false)
  })
})

describe("createSessionLimitStore", () => {
  it("holds the limit a turn ended on", () => {
    const { limit, store } = build()
    limit.emit({ id: "s1", window: "session", resetsAt: 1700 })
    expect(store.get("s1")).toEqual({ window: "session", resetsAt: 1700 })
    expect(store.get("s2")).toBeNull()
  })

  // Codex's end of turn reaches the window after the limit, Claude's before it:
  // neither may take the limit down.
  it("keeps it through the turn's own end", () => {
    const { limit, status, store } = build()
    limit.emit({ id: "s1", window: "session", resetsAt: 1700 })
    status.emit({ id: "s1", state: "interrupted" })
    status.emit({ id: "s1", state: "done" })
    expect(store.get("s1")).not.toBeNull()
  })

  it("clears when a turn starts or the session ends", () => {
    const { limit, status, store } = build()
    limit.emit({ id: "s1", window: "session", resetsAt: 1700 })
    status.emit({ id: "s1", state: "busy" })
    expect(store.get("s1")).toBeNull()

    limit.emit({ id: "s1", window: "session", resetsAt: 1700 })
    status.emit({ id: "s1", state: "idle" })
    expect(store.get("s1")).toBeNull()
  })
})
