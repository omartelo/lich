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

  it("names the missing review behind a blocked pull request", () => {
    expect(
      mergeBlockedReason(
        detail({ mergeStateStatus: "BLOCKED", reviewDecision: "REVIEW_REQUIRED" }),
      ),
    ).toBe("GitHub requires a review that has not been left")
    expect(
      mergeBlockedReason(
        detail({ mergeStateStatus: "BLOCKED", reviewDecision: "CHANGES_REQUESTED" }),
      ),
    ).toBe("A reviewer asked for changes")
  })

  // BLOCKED is reported for any base branch carrying a rule, met or not. A
  // ruleset whose commit-message pattern is only checked against the merge
  // commit holds an approved pull request there for good, and GitHub's own
  // button merges it — so the state alone is not a refusal.
  it("lets an approved pull request through a base branch that has rules", () => {
    expect(
      mergeBlockedReason(detail({ mergeStateStatus: "BLOCKED", reviewDecision: "APPROVED" })),
    ).toBeNull()
  })

  it("lets a rule-bound pull request through where no review is required", () => {
    expect(
      mergeBlockedReason(detail({ mergeStateStatus: "BLOCKED", reviewDecision: "" })),
    ).toBeNull()
  })

  it("blocks a branch the base has moved past", () => {
    expect(mergeBlockedReason(detail({ mergeStateStatus: "BEHIND" }))).toBe(
      "Base branch has moved — update this branch first",
    )
  })

  // GitHub answers UNKNOWN while it recomputes after a push. Waiting is not
  // refusing: the merge is offered, and gh says so if it is early.
  it("lets a merge through while GitHub is still recomputing", () => {
    expect(
      mergeBlockedReason(detail({ mergeStateStatus: "UNKNOWN", mergeable: "UNKNOWN" })),
    ).toBeNull()
  })

  it("lets a state this build has never seen through rather than guessing", () => {
    expect(mergeBlockedReason(detail({ mergeStateStatus: "SOMETHING_NEW" }))).toBeNull()
  })

  it("still catches a conflict a stale merge state hides", () => {
    expect(
      mergeBlockedReason(detail({ mergeStateStatus: "BLOCKED", mergeable: "CONFLICTING" })),
    ).toBe("Conflicts with main")
  })

  it("falls back to mergeable when no merge state came back", () => {
    expect(mergeBlockedReason(detail({ mergeStateStatus: "" }))).toBeNull()
    expect(mergeBlockedReason(detail({ mergeStateStatus: "", mergeable: "CONFLICTING" }))).toBe(
      "Conflicts with main",
    )
  })
})
