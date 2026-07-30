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

  it("files resolved threads separately, newest first", () => {
    const timeline = conversationTimeline(conversation)
    expect(timeline.resolved.map((held) => held.id)).toEqual(["t-done"])
    expect(
      timeline.items.some((item) => item.kind === "thread" && item.thread.id === "t-done"),
    ).toBe(false)
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
