import { describe, expect, it } from "vitest"
import { createSessionTodoStore } from "./session-todo-store"

// harness wires a store to a hand-driven pair of sources.
function harness() {
  let onTodo: (data: unknown) => void = () => {}
  let onStatus: (data: unknown) => void = () => {}
  const store = createSessionTodoStore(
    (h) => {
      onTodo = h
      return () => {}
    },
    (h) => {
      onStatus = h
      return () => {}
    },
  )
  return {
    store,
    todo: (data: unknown) => onTodo(data),
    status: (data: unknown) => onStatus(data),
  }
}

describe("createSessionTodoStore", () => {
  it("has no progress for a session nothing has reported", () => {
    const { store } = harness()
    expect(store.get("s1")).toBeNull()
  })

  it("takes the counts a report carries", () => {
    const { store, todo } = harness()
    todo({ id: "s1", done: 3, total: 7 })
    expect(store.get("s1")).toEqual({ done: 3, total: 7 })
  })

  // The count is over: keeping it would leave a card reading "7 of 7" as news
  // for the rest of the session.
  it("clears when the list is finished", () => {
    const { store, todo } = harness()
    todo({ id: "s1", done: 3, total: 7 })
    todo({ id: "s1", done: 7, total: 7 })
    expect(store.get("s1")).toBeNull()
  })

  // An agent that writes a single item is narrating, not planning.
  it("draws nothing for a list of one", () => {
    const { store, todo } = harness()
    todo({ id: "s1", done: 0, total: 1 })
    expect(store.get("s1")).toBeNull()
  })

  it("ignores a payload from another build", () => {
    const { store, todo } = harness()
    todo({ id: "s1", done: 2, total: 5 })
    todo({ id: "s1", count: 4 })
    expect(store.get("s1")).toBeNull()
  })

  it("keeps sessions apart", () => {
    const { store, todo } = harness()
    todo({ id: "s1", done: 1, total: 4 })
    todo({ id: "s2", done: 3, total: 4 })
    expect(store.get("s1")).toEqual({ done: 1, total: 4 })
    expect(store.get("s2")).toEqual({ done: 3, total: 4 })
  })

  // The store builds a new object per event; a repeat must not hand
  // useSyncExternalStore a new reference and re-render the card for nothing.
  it("holds the same reference for a repeated report", () => {
    const { store, todo } = harness()
    todo({ id: "s1", done: 2, total: 6 })
    const first = store.get("s1")
    todo({ id: "s1", done: 2, total: 6 })
    expect(store.get("s1")).toBe(first)
  })

  // SessionEnd: the provider's CLI left, so the count would sit on a card whose
  // agent is gone.
  it("clears when the session goes idle", () => {
    const { store, todo, status } = harness()
    todo({ id: "s1", done: 2, total: 6 })
    status({ id: "s1", state: "idle" })
    expect(store.get("s1")).toBeNull()
  })

  it("survives every other state report", () => {
    const { store, todo, status } = harness()
    todo({ id: "s1", done: 2, total: 6 })
    status({ id: "s1", state: "busy", tool: "Edit" })
    status({ id: "s1", state: "done" })
    expect(store.get("s1")).toEqual({ done: 2, total: 6 })
  })
})
