import { describe, expect, it } from "vitest"
import type { PullRequestConversation, ReviewThread } from "@/lib/api-types"
import { conversationCount, conversationTimeline } from "./conversation-timeline"

const thread = (id: string, at: string, resolved = false): ReviewThread => ({
  id,
  path: "internal/project/pr.go",
  line: 61,
  startLine: 0,
  side: "RIGHT",
  isResolved: resolved,
  isOutdated: false,
  comments: [{ id: 1, author: "omartelo", body: "here", date: at, diffHunk: "@@" }],
})

const conversation: PullRequestConversation = {
  headRefOid: "abc",
  reviews: [
    {
      author: "omartelo",
      state: "CHANGES_REQUESTED",
      body: "two things",
      date: "2026-07-30T10:00:00Z",
    },
  ],
  comments: [{ author: "omartelo", body: "rebased", date: "2026-07-30T09:00:00Z" }],
  threads: [
    thread("t-late", "2026-07-30T11:00:00Z"),
    thread("t-early", "2026-07-30T08:00:00Z"),
    thread("t-done", "2026-07-30T07:00:00Z", true),
  ],
}

describe("conversationTimeline", () => {
  it("interleaves verdicts, comments and threads by when they were said", () => {
    const timeline = conversationTimeline(conversation)
    expect(timeline.items.map((item) => item.kind)).toEqual([
      "thread", // 08:00
      "comment", // 09:00
      "review", // 10:00
      "thread", // 11:00
    ])
  })

  it("keeps a settled thread out of the timeline", () => {
    const timeline = conversationTimeline(conversation)
    expect(timeline.resolved.map((held) => held.id)).toEqual(["t-done"])
    expect(
      timeline.items.some((item) => item.kind === "thread" && item.thread.id === "t-done"),
    ).toBe(false)
  })

  // Two of them, deliberately: with one settled thread the assertion passes
  // whichever way the comparator runs, so the order it promises goes unchecked.
  it("files settled threads newest first", () => {
    const timeline = conversationTimeline({
      ...conversation,
      threads: [
        thread("t-open", "2026-07-30T08:00:00Z"),
        thread("t-settled-early", "2026-07-30T07:00:00Z", true),
        thread("t-settled-late", "2026-07-30T11:00:00Z", true),
      ],
    })
    expect(timeline.resolved.map((held) => held.id)).toEqual(["t-settled-late", "t-settled-early"])
  })

  it("counts everything the tab shows, settled threads included", () => {
    // One verdict, one comment, two open threads, one settled.
    expect(conversationCount(conversationTimeline(conversation))).toBe(5)
  })

  it("reads a pull request nobody has touched as empty", () => {
    expect(conversationTimeline(null)).toEqual({ items: [], resolved: [] })
    expect(
      conversationTimeline({ headRefOid: "abc", reviews: null, comments: null, threads: null }),
    ).toEqual({ items: [], resolved: [] })
  })

  it("places a thread with no comments left in it rather than dropping it", () => {
    // GitHub keeps the thread when its last comment is deleted; it has no date
    // of its own, so it sorts to the front instead of vanishing from the tab.
    const timeline = conversationTimeline({
      headRefOid: "abc",
      reviews: null,
      comments: [{ author: "someone", body: "later", date: "2026-07-30T12:00:00Z" }],
      threads: [{ ...thread("t-empty", "2026-07-30T08:00:00Z"), comments: null }],
    })
    expect(timeline.items.map((item) => item.kind)).toEqual(["thread", "comment"])
  })

  it("keeps something said with an unreadable timestamp", () => {
    const timeline = conversationTimeline({
      headRefOid: "abc",
      reviews: null,
      comments: [{ author: "someone", body: "no date", date: "not a date" }],
      threads: null,
    })
    expect(timeline.items).toHaveLength(1)
  })
})
