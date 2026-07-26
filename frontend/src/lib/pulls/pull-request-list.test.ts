import { describe, expect, it } from "vitest"
import type { PullRequestSummary } from "@/lib/api-types"
import {
  checkVerdict,
  filterCounts,
  filterPullRequests,
  parsePullsQuery,
  parsePullsSort,
  sortPullRequests,
  updatedAgo,
} from "./pull-request-list"

function pr(over: Partial<PullRequestSummary> = {}): PullRequestSummary {
  return {
    number: 1,
    title: "feat: a thing",
    author: "omartelo",
    state: "OPEN",
    isDraft: false,
    reviewDecision: "",
    headRefName: "feat/thing",
    isCrossRepository: false,
    updatedAt: "2026-07-26T10:00:00Z",
    checks: { passed: 0, failed: 0, pending: 0, total: 0 },
    ...over,
  }
}

/** The filter box's own parse, for the tests that only care about the text. */
const q = (raw = "") => parsePullsQuery(raw)

describe("checkVerdict", () => {
  it("reports no checks apart from a pass", () => {
    expect(checkVerdict(pr())).toBe("none")
    expect(checkVerdict(pr({ checks: { passed: 2, failed: 0, pending: 0, total: 2 } }))).toBe(
      "passed",
    )
  })

  it("lets one failure outrank anything still running", () => {
    const checks = { passed: 3, failed: 1, pending: 2, total: 6 }
    expect(checkVerdict(pr({ checks }))).toBe("failed")
  })

  it("is pending while a check is in flight", () => {
    expect(checkVerdict(pr({ checks: { passed: 1, failed: 0, pending: 1, total: 2 } }))).toBe(
      "pending",
    )
  })
})

describe("filterPullRequests", () => {
  const list = [
    pr({ number: 107, title: "feat: list pull requests", headRefName: "feat/pulls-list" }),
    pr({
      number: 105,
      title: "fix: the poller",
      checks: { passed: 2, failed: 1, pending: 0, total: 3 },
    }),
    pr({ number: 98, title: "spike: notifications", isDraft: true, author: "someone" }),
  ]

  it("keeps everything by default", () => {
    expect(filterPullRequests(list, "all", q())).toHaveLength(3)
  })

  it("splits drafts from ready", () => {
    expect(filterPullRequests(list, "drafts", q()).map((p) => p.number)).toEqual([98])
    expect(filterPullRequests(list, "ready", q()).map((p) => p.number)).toEqual([107, 105])
  })

  it("keeps only a red rollup under failing", () => {
    expect(filterPullRequests(list, "failing", q()).map((p) => p.number)).toEqual([105])
  })

  it("searches the number with or without its hash", () => {
    expect(filterPullRequests(list, "all", q("#105")).map((p) => p.number)).toEqual([105])
    expect(filterPullRequests(list, "all", q("105")).map((p) => p.number)).toEqual([105])
  })

  it("searches title, author and branch, case-insensitively", () => {
    expect(filterPullRequests(list, "all", q("POLLER")).map((p) => p.number)).toEqual([105])
    expect(filterPullRequests(list, "all", q("someone")).map((p) => p.number)).toEqual([98])
    expect(filterPullRequests(list, "all", q("feat/pulls")).map((p) => p.number)).toEqual([107])
  })

  it("combines the quick filter with the search", () => {
    expect(filterPullRequests(list, "drafts", q("spike")).map((p) => p.number)).toEqual([98])
    expect(filterPullRequests(list, "ready", q("spike"))).toEqual([])
  })

  it("ignores surrounding whitespace in the query", () => {
    expect(filterPullRequests(list, "all", q("   ")).map((p) => p.number)).toEqual([107, 105, 98])
    expect(filterPullRequests(list, "all", q("  poller ")).map((p) => p.number)).toEqual([105])
  })
})

describe("filterCounts", () => {
  const list = [
    pr({ number: 1 }),
    pr({ number: 2, isDraft: true }),
    pr({ number: 3, checks: { passed: 0, failed: 2, pending: 0, total: 2 } }),
  ]

  it("counts each filter over the whole list, not the selected one", () => {
    expect(filterCounts(list, q())).toEqual({ all: 3, ready: 2, drafts: 1, failing: 1 })
  })

  // The buttons sit right under the filter box: a count that ignored the query
  // would contradict the rows below it.
  it("counts only what the query left standing", () => {
    expect(filterCounts(list, q("is:draft"))).toEqual({ all: 1, ready: 0, drafts: 1, failing: 0 })
    expect(filterCounts(list, q("#3"))).toEqual({ all: 1, ready: 1, drafts: 0, failing: 1 })
  })

  it("agrees with the rows the same query leaves", () => {
    for (const raw of ["", "is:ready", "#2", "review:approved"]) {
      expect(filterCounts(list, q(raw)).all).toBe(filterPullRequests(list, "all", q(raw)).length)
    }
  })

  it("is all zeroes for an empty list", () => {
    expect(filterCounts([], q())).toEqual({ all: 0, ready: 0, drafts: 0, failing: 0 })
  })
})

describe("sortPullRequests", () => {
  const older = pr({ number: 90, updatedAt: "2026-07-20T10:00:00Z" })
  const newest = pr({ number: 107, updatedAt: "2026-07-26T18:00:00Z" })
  const failing = pr({
    number: 100,
    updatedAt: "2026-07-22T10:00:00Z",
    checks: { passed: 1, failed: 1, pending: 0, total: 2 },
  })
  const list = [older, newest, failing]

  it("does not touch the caller's array", () => {
    const input = [...list]
    sortPullRequests(input, "number")
    expect(input).toEqual(list)
  })

  it("puts the most recent update first by default", () => {
    expect(sortPullRequests(list, "updated").map((p) => p.number)).toEqual([107, 100, 90])
  })

  it("reverses that for oldest", () => {
    expect(sortPullRequests(list, "oldest").map((p) => p.number)).toEqual([90, 100, 107])
  })

  it("ranks by number", () => {
    expect(sortPullRequests(list, "number").map((p) => p.number)).toEqual([107, 100, 90])
  })

  it("lifts a red rollup above a newer green one, then falls back to recency", () => {
    expect(sortPullRequests(list, "failing").map((p) => p.number)).toEqual([100, 107, 90])
  })

  // The order rides on a lexicographic compare of gh's ISO strings rather than
  // on Date.parse, so a timestamp nobody can read sinks instead of poisoning
  // the comparator with NaN.
  it("sinks a timestamp it cannot read instead of scrambling the order", () => {
    const broken = pr({ number: 1, updatedAt: "" })
    expect(sortPullRequests([broken, ...list], "updated").map((p) => p.number)).toEqual([
      107, 100, 90, 1,
    ])
    expect(sortPullRequests([broken, ...list], "oldest").map((p) => p.number)).toEqual([
      1, 90, 100, 107,
    ])
  })

  it("ranks a running check above one that reported nothing", () => {
    const pending = pr({ number: 5, checks: { passed: 0, failed: 0, pending: 1, total: 1 } })
    const quiet = pr({ number: 6 })
    expect(sortPullRequests([quiet, pending], "failing").map((p) => p.number)).toEqual([5, 6])
  })
})

describe("updatedAgo", () => {
  const now = Date.parse("2026-07-26T12:00:00Z")

  it.each([
    ["2026-07-26T11:59:30Z", "now"],
    ["2026-07-26T11:48:00Z", "12m"],
    ["2026-07-26T09:00:00Z", "3h"],
    ["2026-07-24T12:00:00Z", "2d"],
    ["2026-07-05T12:00:00Z", "3w"],
  ])("renders %s as %s", (at, want) => {
    expect(updatedAgo(at, now)).toBe(want)
  })

  it("says nothing for a timestamp it cannot read", () => {
    expect(updatedAgo("", now)).toBe("")
    expect(updatedAgo("yesterday", now)).toBe("")
  })
})

describe("parsePullsSort", () => {
  it("keeps a stored sort this build knows", () => {
    expect(parsePullsSort("failing")).toBe("failing")
  })

  it.each([null, "", "by-vibes"])("falls back to recently updated for %j", (raw) => {
    expect(parsePullsSort(raw)).toBe("updated")
  })
})

describe("parsePullsQuery", () => {
  it("defaults to the open pull requests and no other constraint", () => {
    expect(parsePullsQuery("")).toEqual({
      state: "open",
      review: null,
      draft: null,
      fork: null,
      text: "",
    })
  })

  it.each(["open", "closed", "merged", "all"])("reads is:%s as the state", (state) => {
    expect(parsePullsQuery(`is:${state}`).state).toBe(state)
  })

  it("reads is:draft and is:ready as the two sides of one flag", () => {
    expect(parsePullsQuery("is:draft").draft).toBe(true)
    expect(parsePullsQuery("is:ready").draft).toBe(false)
  })

  it("reads is:fork", () => {
    expect(parsePullsQuery("is:fork").fork).toBe(true)
  })

  it.each([
    ["review:required", "required"],
    ["review:approved", "approved"],
    ["review:changes-requested", "changes-requested"],
    ["review:changes_requested", "changes-requested"],
  ])("reads %s", (raw, want) => {
    expect(parsePullsQuery(raw).review).toBe(want)
  })

  it("is case-insensitive on the qualifier and its value", () => {
    expect(parsePullsQuery("IS:Merged").state).toBe("merged")
  })

  it("combines qualifiers and keeps the rest as text", () => {
    expect(parsePullsQuery("is:merged review:approved poller fix")).toEqual({
      state: "merged",
      review: "approved",
      draft: null,
      fork: null,
      text: "poller fix",
    })
  })

  // A qualifier this build does not know stays text: dropping it would widen
  // the list silently, which reads as a filter that does not work.
  it("keeps an unknown qualifier as text", () => {
    const query = parsePullsQuery("is:spicy label:bug")
    expect(query.state).toBe("open")
    expect(query.text).toBe("is:spicy label:bug")
  })

  // Same contract on the qualifier this build does know by name but not by
  // value: a mistyped review state must not quietly become "no review filter".
  it("keeps an unknown review value as text", () => {
    const query = parsePullsQuery("review:aproved")
    expect(query.review).toBeNull()
    expect(query.text).toBe("review:aproved")
  })

  it("the last state wins", () => {
    expect(parsePullsQuery("is:open is:merged").state).toBe("merged")
  })
})

describe("filtering by qualifier", () => {
  const list = [
    pr({ number: 1, reviewDecision: "APPROVED" }),
    pr({ number: 2, reviewDecision: "CHANGES_REQUESTED" }),
    pr({ number: 3, reviewDecision: "REVIEW_REQUIRED", isDraft: true }),
    pr({ number: 4, reviewDecision: "", isCrossRepository: true }),
  ]

  it("keeps only the asked-for review state", () => {
    expect(filterPullRequests(list, "all", q("review:approved")).map((p) => p.number)).toEqual([1])
    expect(
      filterPullRequests(list, "all", q("review:changes-requested")).map((p) => p.number),
    ).toEqual([2])
    expect(filterPullRequests(list, "all", q("review:required")).map((p) => p.number)).toEqual([3])
  })

  // A repository that requires no review reports "", which is not a review
  // state — asking for one must not match it.
  it("never matches a pull request with no review decision", () => {
    for (const review of ["required", "approved", "changes-requested"]) {
      expect(
        filterPullRequests(list, "all", q(`review:${review}`)).map((p) => p.number),
      ).not.toContain(4)
    }
  })

  it("filters drafts and forks", () => {
    expect(filterPullRequests(list, "all", q("is:draft")).map((p) => p.number)).toEqual([3])
    expect(filterPullRequests(list, "all", q("is:ready")).map((p) => p.number)).toEqual([1, 2, 4])
    expect(filterPullRequests(list, "all", q("is:fork")).map((p) => p.number)).toEqual([4])
  })

  it("ands the qualifiers with the quick filter and the text", () => {
    expect(filterPullRequests(list, "drafts", q("is:ready"))).toEqual([])
    expect(filterPullRequests(list, "all", q("review:approved #2"))).toEqual([])
  })
})
