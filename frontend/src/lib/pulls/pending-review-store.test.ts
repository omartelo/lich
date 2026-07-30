import { beforeEach, describe, expect, it, vi } from "vitest"
import {
  addDraftComment,
  clearPendingReview,
  draftsOnFile,
  editDraftComment,
  parsePendingReview,
  pendingReview,
  removeDraftComment,
  resetPendingReviews,
  setReviewBody,
  subscribePendingReview,
} from "./pending-review-store"

// The suite runs in node, which has no localStorage; the store's whole point is
// that a draft outlives the page, so the storage is stubbed rather than mocked
// away — the round-trip through it is what the tests below check.
const store = new Map<string, string>()

vi.stubGlobal("localStorage", {
  getItem: (key: string) => store.get(key) ?? null,
  setItem: (key: string, value: string) => {
    store.set(key, value)
  },
  removeItem: (key: string) => {
    store.delete(key)
  },
})

const PR = "https://github.com/omartelo/lich/pull/131"
const OTHER = "https://github.com/omartelo/lich/pull/130"

const comment = (path: string, line: number, body: string) => ({
  path,
  line,
  startLine: 0,
  side: "RIGHT",
  body,
})

beforeEach(() => {
  store.clear()
  resetPendingReviews()
})

describe("a review being written", () => {
  it("starts empty and stays a stable reference", () => {
    expect(pendingReview(PR)).toEqual({ body: "", comments: [] })
    expect(pendingReview(PR)).toBe(pendingReview(PR))
  })

  it("collects line comments in the order they were written", () => {
    addDraftComment(PR, comment("a.go", 12, "first"))
    addDraftComment(PR, comment("b.ts", 4, "second"))
    expect(pendingReview(PR).comments.map((c) => c.body)).toEqual(["first", "second"])
  })

  it("keeps two pull requests apart", () => {
    addDraftComment(PR, comment("a.go", 12, "here"))
    expect(pendingReview(OTHER).comments).toHaveLength(0)
  })

  it("edits a comment in place instead of reordering it", () => {
    addDraftComment(PR, comment("a.go", 12, "first"))
    addDraftComment(PR, comment("b.ts", 4, "second"))
    editDraftComment(PR, 0, "first, revised")
    expect(pendingReview(PR).comments.map((c) => c.body)).toEqual(["first, revised", "second"])
  })

  it("drops one comment without touching the rest", () => {
    addDraftComment(PR, comment("a.go", 12, "first"))
    addDraftComment(PR, comment("b.ts", 4, "second"))
    removeDraftComment(PR, 0)
    expect(pendingReview(PR).comments.map((c) => c.body)).toEqual(["second"])
  })

  it("ignores an index that is not there", () => {
    addDraftComment(PR, comment("a.go", 12, "first"))
    removeDraftComment(PR, 4)
    editDraftComment(PR, -1, "x")
    expect(pendingReview(PR).comments).toHaveLength(1)
  })

  it("holds the summary body beside the comments", () => {
    setReviewBody(PR, "two things before this lands")
    addDraftComment(PR, comment("a.go", 12, "here"))
    expect(pendingReview(PR).body).toBe("two things before this lands")
  })

  it("leaves the comments alone when only the summary is written", () => {
    // The diff rebuilds its inline gaps from this array's identity, so typing a
    // summary must not read as the line comments having moved.
    addDraftComment(PR, comment("a.go", 12, "here"))
    const before = pendingReview(PR).comments
    setReviewBody(PR, "typing")
    setReviewBody(PR, "typing a")
    expect(pendingReview(PR).comments).toBe(before)
  })
})

describe("what a lost render must not destroy", () => {
  it("survives the store being emptied — a reload restores the draft", () => {
    setReviewBody(PR, "summary")
    addDraftComment(PR, comment("a.go", 12, "here"))

    // A reload is exactly this: the module's memory is gone, localStorage is not.
    resetPendingReviews()

    expect(pendingReview(PR)).toEqual({
      body: "summary",
      comments: [comment("a.go", 12, "here")],
    })
  })

  it("is gone for good once the review is submitted", () => {
    addDraftComment(PR, comment("a.go", 12, "here"))
    clearPendingReview(PR)
    resetPendingReviews()
    expect(pendingReview(PR).comments).toHaveLength(0)
  })

  it("clearing a pull request nobody wrote on tells nobody", () => {
    let calls = 0
    const stop = subscribePendingReview(() => {
      calls++
    })
    clearPendingReview(PR)
    stop()
    expect(calls).toBe(0)
  })
})

describe("subscribers", () => {
  it("hear every write and nothing else", () => {
    let calls = 0
    const stop = subscribePendingReview(() => {
      calls++
    })
    addDraftComment(PR, comment("a.go", 12, "here"))
    setReviewBody(PR, "summary")
    // Writing the same body again changes nothing, so nobody is told.
    setReviewBody(PR, "summary")
    expect(calls).toBe(2)
    stop()
    addDraftComment(PR, comment("a.go", 13, "after"))
    expect(calls).toBe(2)
  })
})

describe("draftsOnFile", () => {
  it("picks the comments of one file and side, with their review index", () => {
    addDraftComment(PR, comment("a.go", 12, "first"))
    addDraftComment(PR, comment("b.ts", 4, "other file"))
    addDraftComment(PR, { ...comment("a.go", 20, "deleted line"), side: "LEFT" })
    addDraftComment(PR, comment("a.go", 30, "second"))

    const right = draftsOnFile(pendingReview(PR).comments, "a.go", "RIGHT")
    expect(right.map((held) => held.index)).toEqual([0, 3])
    expect(right.map((held) => held.comment.body)).toEqual(["first", "second"])

    const left = draftsOnFile(pendingReview(PR).comments, "a.go", "LEFT")
    expect(left.map((held) => held.index)).toEqual([2])
  })

  it("treats an unset side as the new file", () => {
    addDraftComment(PR, { ...comment("a.go", 12, "first"), side: "" })
    expect(draftsOnFile(pendingReview(PR).comments, "a.go", "RIGHT")).toHaveLength(1)
  })
})

describe("a stored draft from somewhere else", () => {
  it("keeps what is usable", () => {
    expect(
      parsePendingReview({
        body: "summary",
        comments: [{ path: "a.go", line: 3, startLine: 1, side: "LEFT", body: "x" }],
      }),
    ).toEqual({
      body: "summary",
      comments: [{ path: "a.go", line: 3, startLine: 1, side: "LEFT", body: "x" }],
    })
  })

  it("fills what an older shape did not carry", () => {
    expect(parsePendingReview({ comments: [{ path: "a.go", line: 3, body: "x" }] })).toEqual({
      body: "",
      comments: [{ path: "a.go", line: 3, startLine: 0, side: "RIGHT", body: "x" }],
    })
  })

  it("drops a comment with no line to hang off", () => {
    const parsed = parsePendingReview({
      body: "keep me",
      comments: [
        { path: "a.go", body: "no line" },
        { path: "", line: 3, body: "no file" },
        { path: "a.go", line: 0, body: "line zero" },
        { path: "a.go", line: 3, body: "" },
        // Not a comment at all — what a truncated or hand-edited store holds.
        "a string",
        null,
      ],
    })
    expect(parsed).toEqual({ body: "keep me", comments: [] })
  })

  it("reads an empty or foreign draft as no draft at all", () => {
    expect(parsePendingReview({ body: "", comments: [] })).toBeNull()
    expect(parsePendingReview("a string")).toBeNull()
    expect(parsePendingReview(null)).toBeNull()
    expect(parsePendingReview({ body: 4, comments: "no" })).toBeNull()
  })

  it("does not let a broken store break the screen", () => {
    store.set(`lich.pulls.review.${PR}`, "{not json")
    expect(pendingReview(PR)).toEqual({ body: "", comments: [] })
  })
})
