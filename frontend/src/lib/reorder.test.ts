import { describe, expect, it } from "vitest"
import { applyOrder } from "./reorder"

const ids = (items: { id: string }[]) => items.map((i) => i.id)
const items = (...names: string[]) => names.map((id) => ({ id }))

describe("applyOrder", () => {
  it("rearranges items to the given id order", () => {
    const got = applyOrder(items("a", "b", "c"), ["c", "a", "b"])
    expect(got && ids(got)).toEqual(["c", "a", "b"])
  })

  it("keeps the original item objects", () => {
    const list = items("a", "b")
    const got = applyOrder(list, ["b", "a"])
    expect(got?.[0]).toBe(list[1])
  })

  it("rejects an order missing an item", () => {
    expect(applyOrder(items("a", "b", "c"), ["a", "b"])).toBeNull()
  })

  it("rejects an order naming an unknown id", () => {
    expect(applyOrder(items("a", "b"), ["a", "b", "ghost"])).toBeNull()
  })

  it("rejects a repeated id instead of cloning the item", () => {
    expect(applyOrder(items("a", "b"), ["a", "a"])).toBeNull()
  })

  it("accepts an unchanged order", () => {
    const got = applyOrder(items("a", "b"), ["a", "b"])
    expect(got && ids(got)).toEqual(["a", "b"])
  })

  // The sidebar outlives navigation, so a drop resolved against another
  // project's cards must fail rather than reorder the wrong list.
  it("rejects an order naming a different project's items entirely", () => {
    expect(applyOrder(items("politintas-1"), ["skipo-2", "skipo-worktree"])).toBeNull()
  })

  it("does not mutate the input", () => {
    const list = items("a", "b", "c")
    applyOrder(list, ["c", "b", "a"])
    expect(ids(list)).toEqual(["a", "b", "c"])
  })
})
