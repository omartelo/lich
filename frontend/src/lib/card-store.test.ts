import { describe, expect, it, vi } from "vitest"
import { createCardStore } from "./card-store"

describe("card-store", () => {
  it("starts closed", () => {
    expect(createCardStore().isOpen("a")).toBe(false)
  })

  it("open then close flips the flag", () => {
    const store = createCardStore()
    store.open("a")
    expect(store.isOpen("a")).toBe(true)
    store.close("a")
    expect(store.isOpen("a")).toBe(false)
  })

  it("tracks keys independently", () => {
    const store = createCardStore()
    store.open("a")
    expect(store.isOpen("a")).toBe(true)
    expect(store.isOpen("b")).toBe(false)
  })

  it("ignores an empty key", () => {
    const store = createCardStore()
    const listener = vi.fn()
    store.subscribe(listener)
    store.open("")
    expect(store.isOpen("")).toBe(false)
    expect(listener).not.toHaveBeenCalled()
  })

  it("notifies subscribers on open and close", () => {
    const store = createCardStore()
    const listener = vi.fn()
    store.subscribe(listener)
    store.open("a")
    store.close("a")
    expect(listener).toHaveBeenCalledTimes(2)
  })

  it("does not notify when the state is unchanged", () => {
    const store = createCardStore()
    const listener = vi.fn()
    store.subscribe(listener)
    store.open("a")
    store.open("a") // already parked
    store.close("b") // never parked
    expect(listener).toHaveBeenCalledTimes(1)
  })

  it("stops notifying after unsubscribe", () => {
    const store = createCardStore()
    const listener = vi.fn()
    const off = store.subscribe(listener)
    off()
    store.open("a")
    expect(listener).not.toHaveBeenCalled()
  })

  // The whole point of the factory: Settings and Pull request cards must not
  // share a key set or a listener list.
  it("keeps separate stores isolated", () => {
    const one = createCardStore()
    const two = createCardStore()
    const listener = vi.fn()
    two.subscribe(listener)
    one.open("a")
    expect(two.isOpen("a")).toBe(false)
    expect(listener).not.toHaveBeenCalled()
  })
})
