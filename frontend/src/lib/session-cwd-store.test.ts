import {describe, expect, it} from "vitest"
import {createSessionCwdStore} from "./session-cwd-store"

// harness wires a store to a hand-driven event source, returning the emitter.
function harness() {
  let handler: (data: unknown) => void = () => {}
  const store = createSessionCwdStore((h) => {
    handler = h
    return () => {}
  })
  return {store, emit: (data: unknown) => handler(data)}
}

describe("createSessionCwdStore", () => {
  it("returns empty while nothing has been reported", () => {
    const {store} = harness()
    expect(store.get("s1")).toBe("")
  })

  it("keeps the last reported cwd per session", () => {
    const {store, emit} = harness()
    emit({id: "s1", cwd: "/home/user/project"})
    emit({id: "s2", cwd: "/tmp"})
    expect(store.get("s1")).toBe("/home/user/project")
    expect(store.get("s2")).toBe("/tmp")
  })

  it("notifies only the session's subscribers on change", () => {
    const {store, emit} = harness()
    let s1 = 0
    let s2 = 0
    store.subscribe("s1", () => s1++)
    store.subscribe("s2", () => s2++)
    emit({id: "s1", cwd: "/a"})
    expect(s1).toBe(1)
    expect(s2).toBe(0)
  })

  it("stays silent on a repeated cwd", () => {
    const {store, emit} = harness()
    let calls = 0
    store.subscribe("s1", () => calls++)
    emit({id: "s1", cwd: "/a"})
    emit({id: "s1", cwd: "/a"})
    expect(calls).toBe(1)
  })

  it("retains the cwd across an unsubscribe, like a card unmount", () => {
    const {store, emit} = harness()
    const off = store.subscribe("s1", () => {})
    emit({id: "s1", cwd: "/a"})
    off()
    expect(store.get("s1")).toBe("/a")
  })

  it("ignores malformed payloads", () => {
    const {store, emit} = harness()
    emit({id: "s1"})
    emit({cwd: "/a"})
    emit(null)
    expect(store.get("s1")).toBe("")
  })

  it("overwrites a stale cwd when the backend re-reports on respawn", () => {
    const {store, emit} = harness()
    emit({id: "s1", cwd: "/somewhere/deep"})
    emit({id: "s1", cwd: "/home/user/project"})
    expect(store.get("s1")).toBe("/home/user/project")
  })
})
