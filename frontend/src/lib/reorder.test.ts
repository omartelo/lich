import { describe, expect, it } from "vitest"
import { applyOrder, pinFirst } from "./reorder"

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

describe("pinFirst", () => {
  it("moves the pinned id to the front", () => {
    expect(pinFirst(["a", "b", "home"], "home")).toEqual(["home", "a", "b"])
  })

  it("adds the pinned id when the drop omitted it (the common case)", () => {
    // ProjectTabs excludes Home from the drag list, so a drop names only a,b.
    expect(pinFirst(["a", "b"], "home")).toEqual(["home", "a", "b"])
  })

  it("deduplicates rather than repeating the pinned id", () => {
    expect(pinFirst(["home", "a"], "home")).toEqual(["home", "a"])
  })

  it("leaves ids untouched when nothing is pinned", () => {
    expect(pinFirst(["a", "b"], null)).toEqual(["a", "b"])
  })

  it("produces a list applyOrder accepts against the full project set", () => {
    const projects = items("home", "a", "b")
    const full = pinFirst(["b", "a"], "home") // Home spliced back onto a reordered drop
    const got = applyOrder(projects, full)
    expect(got && ids(got)).toEqual(["home", "b", "a"])
  })
})
