import { describe, expect, it, vi } from "vitest"
import {
  addReviewComment,
  clearReviewComments,
  composeReviewComments,
  removeReviewComment,
  reviewComments,
  subscribeReviewComments,
} from "./review-comments"

// The store is module-level, so every test writes under its own target.
const target = (name: string) => `/tmp/review/${name}`

describe("review-comments", () => {
  it("starts with nothing pending", () => {
    expect(reviewComments(target("empty"))).toEqual([])
  })

  it("keeps comments in the order they were written", () => {
    const t = target("order")
    addReviewComment(t, "a.ts", "12-30", "leaks the listener")
    addReviewComment(t, "b.ts", "4", "rename this")
    expect(reviewComments(t).map((c) => c.path)).toEqual(["a.ts", "b.ts"])
  })

  it("keeps reviews apart", () => {
    addReviewComment(target("one"), "a.ts", "1", "note")
    expect(reviewComments(target("two"))).toEqual([])
  })

  it("drops blank text and trims the rest", () => {
    const t = target("blank")
    addReviewComment(t, "a.ts", "1", "   \n  ")
    expect(reviewComments(t)).toEqual([])
    addReviewComment(t, "a.ts", "1", "  note  ")
    expect(reviewComments(t)[0].text).toBe("note")
  })

  it("removes one comment and clears the rest", () => {
    const t = target("remove")
    addReviewComment(t, "a.ts", "1", "first")
    addReviewComment(t, "b.ts", "2", "second")
    removeReviewComment(t, reviewComments(t)[0].id)
    expect(reviewComments(t).map((c) => c.text)).toEqual(["second"])
    clearReviewComments(t)
    expect(reviewComments(t)).toEqual([])
  })

  it("notifies subscribers on a change, and never on a no-op", () => {
    const t = target("notify")
    const listener = vi.fn()
    const unsubscribe = subscribeReviewComments(listener)
    addReviewComment(t, "a.ts", "1", "note")
    expect(listener).toHaveBeenCalledTimes(1)
    removeReviewComment(t, "no-such-id")
    clearReviewComments(target("never-written"))
    expect(listener).toHaveBeenCalledTimes(1)
    unsubscribe()
    addReviewComment(t, "b.ts", "2", "another")
    expect(listener).toHaveBeenCalledTimes(1)
  })

  it("composes one paste holding every comment, anchored like an inject", () => {
    const t = target("compose")
    addReviewComment(t, "src/a.ts", "12-30", "leaks the listener")
    addReviewComment(t, "src/b.tsx", "44", "rename to X")
    const payload = composeReviewComments(reviewComments(t))
    expect(payload).toBe(
      "\x1b[200~Review comments:\n\n" +
        "- src/a.ts:12-30 — leaks the listener\n" +
        "- src/b.tsx:44 — rename to X\x1b[201~",
    )
  })

  it("keeps a multi-line comment as one list item", () => {
    const t = target("multiline")
    addReviewComment(t, "a.ts", "1", "first\nsecond")
    expect(composeReviewComments(reviewComments(t))).toContain("- a.ts:1 — first\n  second")
  })
})
