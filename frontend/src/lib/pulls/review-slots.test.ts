import { describe, expect, it } from "vitest"
import type { DraftReviewComment, ReviewThread } from "@/lib/api-types"
import { buildFileDoc, parseDiff } from "@/lib/git/diff"
import { COMPOSER_KEY, draftSlotKey, isAnchored, reviewSlots, threadSlotKey } from "./review-slots"

// doc: 1 context(1/1) · 2 del(old 2) · 3 add(new 2) · 4 add(new 3) · 5 context(3/4)
//      6 separator · 7 del(old 10) · 8 add(new 11) · 9 context(11/12)
const diff = `diff --git a/src/app.ts b/src/app.ts
--- a/src/app.ts
+++ b/src/app.ts
@@ -1,3 +1,4 @@
 const a = 1
-const b = 2
+const b = 3
+const c = 4
 export {}
@@ -10,2 +11,2 @@ function tail() {
-  return b
+  return c
   }`

const lineMeta = buildFileDoc(parseDiff(diff)[0]).lineMeta

const thread = (id: string, line: number, over: Partial<ReviewThread> = {}): ReviewThread => ({
  id,
  path: "src/app.ts",
  line,
  startLine: 0,
  side: "RIGHT",
  isResolved: false,
  isOutdated: false,
  comments: null,
  ...over,
})

const draft = (line: number, over: Partial<DraftReviewComment> = {}): DraftReviewComment => ({
  path: "src/app.ts",
  line,
  startLine: 0,
  side: "RIGHT",
  body: "here",
  ...over,
})

const slots = (over: Partial<Parameters<typeof reviewSlots>[0]> = {}) =>
  reviewSlots({ lineMeta, threads: [], drafts: [], ...over })

describe("reviewSlots", () => {
  it("anchors a thread on the document line rendering its file line", () => {
    expect(slots({ threads: [thread("a", 3)] })).toEqual([{ key: "t:a", docLine: 4 }])
  })

  it("anchors a thread on a deleted line to the other side of the diff", () => {
    expect(slots({ threads: [thread("a", 10, { side: "LEFT" })] })).toEqual([
      { key: "t:a", docLine: 7 },
    ])
  })

  it("leaves out a thread the diff cannot hold", () => {
    // Outdated: the line number addresses code this diff no longer shows.
    expect(slots({ threads: [thread("a", 3, { isOutdated: true })] })).toEqual([])
    // On a line outside every hunk — untouched code the diff never rendered.
    expect(slots({ threads: [thread("a", 500)] })).toEqual([])
  })

  it("still anchors a resolved thread, so closing one does not hide it", () => {
    expect(slots({ threads: [thread("a", 3, { isResolved: true })] })).toHaveLength(1)
  })

  it("gives the comments on one line a single gap", () => {
    const written = [draft(3), draft(3), draft(2)]
    expect(slots({ drafts: written })).toEqual([
      { key: "d:RIGHT:3-3", docLine: 4 },
      { key: "d:RIGHT:2-2", docLine: 3 },
    ])
  })

  it("keys a range by both ends, so it never shares a gap with a single line", () => {
    expect(draftSlotKey(draft(3, { startLine: 2 }))).toBe("d:RIGHT:2-3")
    expect(draftSlotKey(draft(3))).toBe("d:RIGHT:3-3")
  })

  it("keeps a thread and a draft on the same line in separate gaps", () => {
    const built = slots({ threads: [thread("a", 3)], drafts: [draft(3)] })
    expect(built.map((slot) => slot.key)).toEqual(["t:a", "d:RIGHT:3-3"])
    expect(built.every((slot) => slot.docLine === 4)).toBe(true)
  })

  it("leaves the composer to the caller", () => {
    // It hangs off the document line the selection ended on — which the caller
    // has and this does not, and which exists even for a selection that ends on
    // a deleted line and so has no new-file line at all.
    expect(slots().some((slot) => slot.key === COMPOSER_KEY)).toBe(false)
  })

  it("never emits the same key twice", () => {
    const twice = slots({ threads: [thread("a", 3), thread("a", 3)] })
    expect(twice).toHaveLength(1)
  })
})

describe("threadSlotKey", () => {
  it("keys a thread by its node id, which outlives its line moving", () => {
    expect(threadSlotKey(thread("PRRT_kw1", 3))).toBe("t:PRRT_kw1")
  })
})

describe("isAnchored", () => {
  it("answers what the diff can render, so the tab can show the rest", () => {
    expect(isAnchored(thread("a", 3), lineMeta)).toBe(true)
    expect(isAnchored(thread("a", 3, { isOutdated: true }), lineMeta)).toBe(false)
    expect(isAnchored(thread("a", 900), lineMeta)).toBe(false)
  })
})
