import { describe, expect, it, vi } from "vitest"
import { createSessionToolStore } from "./session-tool-store"

// harness wires a store to a hand-driven status source.
function harness() {
  let onStatus: (data: unknown) => void = () => {}
  const store = createSessionToolStore((h) => {
    onStatus = h
    return () => {}
  })
  return { store, status: (data: unknown) => onStatus(data) }
}

describe("createSessionToolStore", () => {
  it("has no tool for a session nothing has reported", () => {
    const { store } = harness()
    expect(store.get("s1")).toBeNull()
  })

  it("takes the tool a busy report names", () => {
    const { store, status } = harness()
    status({ id: "s1", state: "busy", tool: "Bash", detail: "pnpm test" })
    expect(store.get("s1")).toEqual({ name: "Bash", detail: "pnpm test" })
  })

  it("keeps each harness's own word for the tool", () => {
    const { store, status } = harness()
    status({ id: "s1", state: "busy", tool: "apply_patch", detail: "usage.go" })
    expect(store.get("s1")).toEqual({ name: "apply_patch", detail: "usage.go" })
  })

  it("takes a tool with no detail", () => {
    const { store, status } = harness()
    status({ id: "s1", state: "busy", tool: "Read" })
    expect(store.get("s1")).toEqual({ name: "Read", detail: "" })
  })

  // PostToolUse reports busy between every two tools. Clearing there would
  // blink the line off and back on at every step of a turn.
  it("holds the last tool through a busy that names none", () => {
    const { store, status } = harness()
    status({ id: "s1", state: "busy", tool: "Bash", detail: "pnpm test" })
    status({ id: "s1", state: "busy" })
    expect(store.get("s1")).toEqual({ name: "Bash", detail: "pnpm test" })
  })

  it("replaces the tool when the next one is reported", () => {
    const { store, status } = harness()
    status({ id: "s1", state: "busy", tool: "Bash", detail: "pnpm test" })
    status({ id: "s1", state: "busy", tool: "Edit", detail: "SessionCard.tsx" })
    expect(store.get("s1")).toEqual({ name: "Edit", detail: "SessionCard.tsx" })
  })

  it.each(["done", "waiting", "idle"])("drops the tool on %s", (state) => {
    const { store, status } = harness()
    status({ id: "s1", state: "busy", tool: "Bash", detail: "pnpm test" })
    status({ id: "s1", state })
    expect(store.get("s1")).toBeNull()
  })

  // Codex has no SessionEnd, so "idle" never arrives there: leaving busy is
  // what has to clear the line, not idle alone.
  it("drops the tool on a state it does not know", () => {
    const { store, status } = harness()
    status({ id: "s1", state: "busy", tool: "Bash", detail: "pnpm test" })
    status({ id: "s1", state: "compacting" })
    expect(store.get("s1")).toBeNull()
  })

  it("keeps sessions apart", () => {
    const { store, status } = harness()
    status({ id: "s1", state: "busy", tool: "Bash", detail: "pnpm test" })
    status({ id: "s2", state: "busy", tool: "apply_patch", detail: "usage.go" })
    status({ id: "s2", state: "done" })
    expect(store.get("s1")).toEqual({ name: "Bash", detail: "pnpm test" })
    expect(store.get("s2")).toBeNull()
  })

  it("ignores a payload that is not a status event", () => {
    const { store, status } = harness()
    status({ id: "s1", state: "busy", tool: "Bash", detail: "pnpm test" })
    status({ id: "s1" })
    status({ state: "done" })
    status(null)
    status("busy")
    expect(store.get("s1")).toEqual({ name: "Bash", detail: "pnpm test" })
  })

  it("ignores a tool that is not a string", () => {
    const { store, status } = harness()
    status({ id: "s1", state: "busy", tool: 42, detail: "pnpm test" })
    expect(store.get("s1")).toBeNull()
  })

  it("treats a detail that is not a string as none", () => {
    const { store, status } = harness()
    status({ id: "s1", state: "busy", tool: "Bash", detail: { cmd: "pnpm test" } })
    expect(store.get("s1")).toEqual({ name: "Bash", detail: "" })
  })

  // The store rebuilds the object on every report, so without an equality guard
  // a repeat would hand useSyncExternalStore a new reference every time.
  it("does not notify when the same tool is reported again", () => {
    const { store, status } = harness()
    const listener = vi.fn()
    store.subscribe("s1", listener)
    status({ id: "s1", state: "busy", tool: "Bash", detail: "pnpm test" })
    expect(listener).toHaveBeenCalledTimes(1)
    status({ id: "s1", state: "busy", tool: "Bash", detail: "pnpm test" })
    expect(listener).toHaveBeenCalledTimes(1)
  })

  it("notifies when only the detail moves", () => {
    const { store, status } = harness()
    const listener = vi.fn()
    store.subscribe("s1", listener)
    status({ id: "s1", state: "busy", tool: "Bash", detail: "pnpm test" })
    status({ id: "s1", state: "busy", tool: "Bash", detail: "pnpm build" })
    expect(listener).toHaveBeenCalledTimes(2)
    expect(store.get("s1")).toEqual({ name: "Bash", detail: "pnpm build" })
  })
})
