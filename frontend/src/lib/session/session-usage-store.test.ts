import { describe, expect, it } from "vitest"
import { createSessionUsageStore } from "./session-usage-store"

// harness wires a store to a hand-driven event source, returning the emitter.
function harness() {
  let handler: (data: unknown) => void = () => {}
  const store = createSessionUsageStore((h) => {
    handler = h
    return () => {}
  })
  return { store, emit: (data: unknown) => handler(data) }
}

describe("createSessionUsageStore", () => {
  it("returns null while nothing has been reported", () => {
    const { store } = harness()
    expect(store.get("s1")).toBeNull()
  })

  const opus = "claude-opus-4-8"
  const eff = "high"

  it("keeps the last reported usage per session", () => {
    const { store, emit } = harness()
    emit({ id: "s1", percent: 42, tokens: 84000, window: 200000, model: opus, effort: eff })
    emit({ id: "s2", percent: 5, tokens: 50000, window: 1000000, model: opus, effort: eff })
    expect(store.get("s1")).toEqual({
      percent: 42,
      tokens: 84000,
      window: 200000,
      model: opus,
      effort: eff,
    })
    expect(store.get("s2")).toEqual({
      percent: 5,
      tokens: 50000,
      window: 1000000,
      model: opus,
      effort: eff,
    })
  })

  it("notifies only the session's subscribers on change", () => {
    const { store, emit } = harness()
    let s1 = 0
    let s2 = 0
    store.subscribe("s1", () => s1++)
    store.subscribe("s2", () => s2++)
    emit({ id: "s1", percent: 10, tokens: 20000, window: 200000, model: opus, effort: eff })
    expect(s1).toBe(1)
    expect(s2).toBe(0)
  })

  it("stays silent, and keeps the reference, on an unchanged report", () => {
    const { store, emit } = harness()
    let calls = 0
    store.subscribe("s1", () => calls++)
    emit({ id: "s1", percent: 10, tokens: 20000, window: 200000, model: opus, effort: eff })
    const first = store.get("s1")
    emit({ id: "s1", percent: 10, tokens: 20000, window: 200000, model: opus, effort: eff })
    expect(calls).toBe(1)
    expect(store.get("s1")).toBe(first)
  })

  it("re-renders when the effort changes at equal tokens", () => {
    const { store, emit } = harness()
    let calls = 0
    store.subscribe("s1", () => calls++)
    emit({ id: "s1", percent: 10, tokens: 20000, window: 200000, model: opus, effort: "high" })
    emit({ id: "s1", percent: 10, tokens: 20000, window: 200000, model: opus, effort: "xhigh" })
    expect(calls).toBe(2)
    expect(store.get("s1")).toEqual({
      percent: 10,
      tokens: 20000,
      window: 200000,
      model: opus,
      effort: "xhigh",
    })
  })

  it("retains the usage across an unsubscribe, like a card unmount", () => {
    const { store, emit } = harness()
    const off = store.subscribe("s1", () => {})
    emit({ id: "s1", percent: 30, tokens: 60000, window: 200000, model: opus, effort: eff })
    off()
    expect(store.get("s1")).toEqual({
      percent: 30,
      tokens: 60000,
      window: 200000,
      model: opus,
      effort: eff,
    })
  })

  it("ignores malformed payloads", () => {
    const { store, emit } = harness()
    emit({ id: "s1", percent: 10, tokens: 20000, window: 200000, model: opus })
    emit({ id: "s1", tokens: 20000, window: 200000, model: opus, effort: eff })
    emit({ percent: 10, tokens: 20000, window: 200000, model: opus, effort: eff })
    emit(null)
    expect(store.get("s1")).toBeNull()
  })
})
