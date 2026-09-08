import { describe, expect, it } from "vitest"
import { createSessionCwdStore } from "./session-cwd-store"

// harness wires a store to a hand-driven event source, returning the emitter.
function harness() {
  let handler: (data: unknown) => void = () => {}
  const store = createSessionCwdStore((h) => {
    handler = h
    return () => {}
  })
  return { store, emit: (data: unknown) => handler(data) }
}

describe("createSessionCwdStore", () => {
  it("returns nothing while nothing has been reported", () => {
    const { store } = harness()
    expect(store.get("s1")).toEqual({ cwd: "", host: "" })
  })

  it("keeps the last reported cwd per session", () => {
    const { store, emit } = harness()
    emit({ id: "s1", cwd: "/home/user/project", host: "" })
    emit({ id: "s2", cwd: "/tmp", host: "" })
    expect(store.get("s1")).toEqual({ cwd: "/home/user/project", host: "" })
    expect(store.get("s2")).toEqual({ cwd: "/tmp", host: "" })
  })

  it("notifies only the session's subscribers on change", () => {
    const { store, emit } = harness()
    let s1 = 0
    let s2 = 0
    store.subscribe("s1", () => s1++)
    store.subscribe("s2", () => s2++)
    emit({ id: "s1", cwd: "/a", host: "" })
    expect(s1).toBe(1)
    expect(s2).toBe(0)
  })

  it("stays silent on a repeated cwd", () => {
    const { store, emit } = harness()
    let calls = 0
    store.subscribe("s1", () => calls++)
    emit({ id: "s1", cwd: "/a", host: "" })
    emit({ id: "s1", cwd: "/a", host: "" })
    expect(calls).toBe(1)
  })

  it("retains the cwd across an unsubscribe, like a card unmount", () => {
    const { store, emit } = harness()
    const off = store.subscribe("s1", () => {})
    emit({ id: "s1", cwd: "/a", host: "" })
    off()
    expect(store.get("s1")).toEqual({ cwd: "/a", host: "" })
  })

  it("ignores malformed payloads", () => {
    const { store, emit } = harness()
    emit({ id: "s1" })
    emit({ cwd: "/a" })
    emit({ id: "s1", cwd: "/a", host: 3 })
    emit(null)
    expect(store.get("s1")).toEqual({ cwd: "", host: "" })
  })

  it("overwrites a stale cwd when the backend re-reports on respawn", () => {
    const { store, emit } = harness()
    emit({ id: "s1", cwd: "/somewhere/deep", host: "" })
    emit({ id: "s1", cwd: "/home/user/project", host: "" })
    expect(store.get("s1")).toEqual({ cwd: "/home/user/project", host: "" })
  })

  it("drops the path it knew when the shell moves out of reach", () => {
    const { store, emit } = harness()
    emit({ id: "s1", cwd: "/home/user/project", host: "" })
    emit({ id: "s1", cwd: "", host: "tmux" })
    // The old path must not survive as a fallback: it is a real directory, and
    // the card would draw it as though the session were still standing there.
    expect(store.get("s1")).toEqual({ cwd: "", host: "tmux" })
  })

  it("notifies when only the host changes", () => {
    const { store, emit } = harness()
    let calls = 0
    store.subscribe("s1", () => calls++)
    emit({ id: "s1", cwd: "", host: "tmux" })
    emit({ id: "s1", cwd: "", host: "ssh" })
    expect(calls).toBe(2)
    expect(store.get("s1")).toEqual({ cwd: "", host: "ssh" })
  })
})
