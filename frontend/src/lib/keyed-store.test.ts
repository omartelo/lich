import { describe, expect, it } from "vitest"
import { createKeyedStore } from "./keyed-store"

describe("createKeyedStore", () => {
  it("answers the empty value for a key nothing has reported", () => {
    expect(createKeyedStore("").get("a")).toBe("")
    expect(createKeyedStore<number | null>(null).get("a")).toBeNull()
  })

  it("keeps a value per key", () => {
    const store = createKeyedStore("")
    store.set("a", "one")
    store.set("b", "two")
    expect(store.get("a")).toBe("one")
    expect(store.get("b")).toBe("two")
  })

  it("notifies only the key's own subscribers", () => {
    const store = createKeyedStore("")
    let a = 0
    let b = 0
    store.subscribe("a", () => a++)
    store.subscribe("b", () => b++)
    store.set("a", "one")
    expect(a).toBe(1)
    expect(b).toBe(0)
  })

  it("stays silent on a value equal to the current one", () => {
    const store = createKeyedStore("")
    let calls = 0
    store.subscribe("a", () => calls++)
    store.set("a", "one")
    store.set("a", "one")
    expect(calls).toBe(1)
  })

  it("takes a custom comparison for values rebuilt on every report", () => {
    const store = createKeyedStore<{ n: number }>({ n: 0 }, (x, y) => x.n === y.n)
    let calls = 0
    store.subscribe("a", () => calls++)
    store.set("a", { n: 1 })
    const first = store.get("a")
    // A fresh object holding the same field must neither notify nor replace the
    // snapshot reference useSyncExternalStore compares.
    store.set("a", { n: 1 })
    expect(calls).toBe(1)
    expect(store.get("a")).toBe(first)
  })

  it("retains the value across an unsubscribe, like a card unmount", () => {
    const store = createKeyedStore("")
    const off = store.subscribe("a", () => {})
    store.set("a", "one")
    off()
    expect(store.get("a")).toBe("one")
  })

  it("drops only the listener that unsubscribes", () => {
    const store = createKeyedStore("")
    let kept = 0
    const off = store.subscribe("a", () => {})
    store.subscribe("a", () => kept++)
    off()
    store.set("a", "one")
    expect(kept).toBe(1)
  })

  it("reaches a subscriber that arrived after the value did", () => {
    const store = createKeyedStore("")
    store.set("a", "one")
    let calls = 0
    store.subscribe("a", () => calls++)
    expect(store.get("a")).toBe("one")
    store.set("a", "two")
    expect(calls).toBe(1)
  })
})
