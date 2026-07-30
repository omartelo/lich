import { describe, expect, it } from "vitest"
import { mergeBlockedReason } from "./merge-gate"
import type { PullRequestDetail } from "@/lib/api-types"

const detail = (over: Partial<PullRequestDetail> = {}): PullRequestDetail => ({
  number: 130,
  url: "https://github.com/o/l/pull/130",
  title: "feat: something",
  body: "",
  state: "OPEN",
  isDraft: false,
  mergeable: "MERGEABLE",
  mergeStateStatus: "CLEAN",
  reviewDecision: "",
  baseRefName: "main",
  headRefName: "quiet-willow",
  changedFiles: 1,
  isCrossRepository: false,
  checks: { total: 0, passed: 0, failed: 0, pending: 0 },
  checkRuns: null,
  commits: null,
  ...over,
})

describe("mergeBlockedReason", () => {
  it("lets a clean pull request through", () => {
    expect(mergeBlockedReason(detail())).toBeNull()
  })

  it("lets a merge over a non-required red check through", () => {
    expect(mergeBlockedReason(detail({ mergeStateStatus: "UNSTABLE" }))).toBeNull()
  })

  it("lets a repository with merge hooks through", () => {
    expect(mergeBlockedReason(detail({ mergeStateStatus: "HAS_HOOKS" }))).toBeNull()
  })

  it("blocks a pull request that is already over", () => {
    expect(mergeBlockedReason(detail({ state: "MERGED" }))).toBe("Pull request is merged")
    expect(mergeBlockedReason(detail({ state: "CLOSED" }))).toBe("Pull request is closed")
  })

  it("blocks a draft before reading the merge state", () => {
    expect(mergeBlockedReason(detail({ isDraft: true, mergeStateStatus: "DRAFT" }))).toBe(
      "Pull request is a draft",
    )
  })

  it("names the base branch on a conflict", () => {
    const reason = mergeBlockedReason(
      detail({ mergeStateStatus: "DIRTY", mergeable: "CONFLICTING", baseRefName: "develop" }),
    )
    expect(reason).toBe("Conflicts with develop")
  })

  it("blocks an unmet review or required check", () => {
    expect(mergeBlockedReason(detail({ mergeStateStatus: "BLOCKED" }))).toBe(
      "GitHub requires a review or a status check that has not passed",
    )
  })

  it("blocks a branch the base has moved past", () => {
    expect(mergeBlockedReason(detail({ mergeStateStatus: "BEHIND" }))).toBe(
      "Base branch has moved — update this branch first",
    )
  })

  // The state that produced the flat "not mergeable" toast: GitHub had not
  // finished recomputing after a push, and the old gate only looked at
  // mergeable === CONFLICTING.
  it("blocks while GitHub is still recomputing", () => {
    expect(mergeBlockedReason(detail({ mergeStateStatus: "UNKNOWN", mergeable: "UNKNOWN" }))).toBe(
      "GitHub is still working out whether this can merge",
    )
  })

  it("blocks a state this build has never seen", () => {
    expect(mergeBlockedReason(detail({ mergeStateStatus: "SOMETHING_NEW" }))).toBe(
      "GitHub will not merge this pull request yet",
    )
  })

  it("falls back to mergeable when no merge state came back", () => {
    expect(mergeBlockedReason(detail({ mergeStateStatus: "" }))).toBeNull()
    expect(mergeBlockedReason(detail({ mergeStateStatus: "", mergeable: "CONFLICTING" }))).toBe(
      "Conflicts with main",
    )
  })
})
