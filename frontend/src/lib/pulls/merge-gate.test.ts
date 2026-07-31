import { describe, expect, it } from "vitest"
import {
  allowedMergeMethods,
  canAdminOverride,
  mergeBlockedReason,
  mergeRuleNote,
} from "./merge-gate"
import type { BranchRules, PullRequestDetail } from "@/lib/api-types"

const rules = (over: Partial<BranchRules> = {}): BranchRules => ({
  allowedMergeMethods: null,
  commitPatterns: null,
  viewerCanBypass: false,
  ...over,
})

const admin = (over: Partial<BranchRules> = {}) => rules({ viewerCanBypass: true, ...over })

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

describe("allowedMergeMethods", () => {
  it("narrows the menu to what the base branch's ruleset accepts", () => {
    expect(allowedMergeMethods(rules({ allowedMergeMethods: ["squash"] }))).toEqual(["squash"])
  })

  it("keeps the menu's own order, not the ruleset's", () => {
    expect(allowedMergeMethods(rules({ allowedMergeMethods: ["rebase", "squash"] }))).toEqual([
      "squash",
      "rebase",
    ])
  })

  // Every unknown widens. A menu that lost an option lich could have offered
  // says nothing about why; one that offers an option GitHub turns down at
  // least answers with gh's own refusal.
  it("offers everything where nothing is known", () => {
    const all = ["squash", "merge", "rebase"]
    // The call failed, or has not answered yet.
    expect(allowedMergeMethods(null)).toEqual(all)
    // The branch has no ruleset, or one that says nothing about merging.
    expect(allowedMergeMethods(rules({ allowedMergeMethods: [] }))).toEqual(all)
    expect(allowedMergeMethods(rules({ allowedMergeMethods: null }))).toEqual(all)
    // A ruleset naming only methods this build cannot ask gh for.
    expect(allowedMergeMethods(rules({ allowedMergeMethods: ["fast_forward"] }))).toEqual(all)
  })
})

describe("canAdminOverride", () => {
  // The two gh itself allows --admin on; anything else and gh refuses the flag
  // as well, so offering it would trade one dead click for another.
  it("offers the bypass on the states GitHub lets an administrator past", () => {
    expect(canAdminOverride(detail({ mergeStateStatus: "BLOCKED" }), admin())).toBe(true)
    expect(canAdminOverride(detail({ mergeStateStatus: "BEHIND" }), admin())).toBe(true)
  })

  // The rest of this module widens on an unknown; this one hides. A bypass
  // reads as a privilege you hold, so offering one that is not there turns
  // GitHub's refusal into what looks like a broken button.
  it("hides the bypass from an account that does not administer the repository", () => {
    const blocked = detail({ mergeStateStatus: "BLOCKED" })
    expect(canAdminOverride(blocked, rules())).toBe(false)
    // The rules have not answered yet, or the read failed.
    expect(canAdminOverride(blocked, null)).toBe(false)
  })

  it("does not offer it where nothing is in the way", () => {
    expect(canAdminOverride(detail(), admin())).toBe(false)
    expect(canAdminOverride(detail({ mergeStateStatus: "UNSTABLE" }), admin())).toBe(false)
  })

  // An override skips a rule; it does not resolve a conflict or finish a draft.
  it("does not offer it for what an override cannot fix", () => {
    expect(
      canAdminOverride(detail({ mergeStateStatus: "DIRTY", mergeable: "CONFLICTING" }), admin()),
    ).toBe(false)
    // The conflict a stale merge state hides — BLOCKED is on the override list,
    // and only the second field says there is nothing to override.
    expect(
      canAdminOverride(detail({ mergeStateStatus: "BLOCKED", mergeable: "CONFLICTING" }), admin()),
    ).toBe(false)
    expect(canAdminOverride(detail({ isDraft: true, mergeStateStatus: "BLOCKED" }), admin())).toBe(
      false,
    )
    expect(
      canAdminOverride(detail({ state: "MERGED", mergeStateStatus: "BLOCKED" }), admin()),
    ).toBe(false)
  })
})

describe("mergeRuleNote", () => {
  const pattern = {
    target: "message",
    operator: "regex",
    pattern: "^(feat|fix): .+",
    negate: false,
  }

  // The case the note exists for: approved, green, no conflict, still BLOCKED.
  it("names the commit rule governing the base branch", () => {
    expect(
      mergeRuleNote(
        detail({ mergeStateStatus: "BLOCKED", reviewDecision: "APPROVED", baseRefName: "develop" }),
        rules({ commitPatterns: [pattern] }),
      ),
    ).toBe("A ruleset governs develop: every commit message must match ^(feat|fix): .+.")
  })

  it("reads a negated rule as the prohibition it is", () => {
    expect(
      mergeRuleNote(
        detail({ mergeStateStatus: "BLOCKED" }),
        rules({
          commitPatterns: [
            { target: "author email", operator: "ends_with", pattern: "@ex.com", negate: true },
          ],
        }),
      ),
    ).toBe("A ruleset governs main: every commit author email must not end with @ex.com.")
  })

  it("names every rule when the branch carries more than one", () => {
    const note = mergeRuleNote(
      detail({ mergeStateStatus: "BLOCKED" }),
      rules({
        commitPatterns: [
          pattern,
          { target: "committer email", operator: "contains", pattern: "@", negate: false },
        ],
      }),
    )
    expect(note).toContain("message must match ^(feat|fix): .+")
    expect(note).toContain("committer email must contain @")
  })

  // An operator GitHub adds after this build shipped still gets its pattern on
  // screen — a slightly wrong verb beats no note at all.
  it("falls back to a verb for an operator it has never seen", () => {
    expect(
      mergeRuleNote(
        detail({ mergeStateStatus: "BLOCKED" }),
        rules({ commitPatterns: [{ ...pattern, operator: "SOMETHING_NEW" }] }),
      ),
    ).toContain("message must match ^(feat|fix): .+")
  })

  it("says nothing when GitHub is not holding the merge back", () => {
    expect(mergeRuleNote(detail(), rules({ commitPatterns: [pattern] }))).toBeNull()
  })

  // A rule note is not a verdict: no rule of this kind never means "unblocked".
  it("says nothing when no rule of this kind is known", () => {
    const blocked = detail({ mergeStateStatus: "BLOCKED" })
    expect(mergeRuleNote(blocked, null)).toBeNull()
    expect(mergeRuleNote(blocked, rules())).toBeNull()
    expect(mergeRuleNote(blocked, rules({ commitPatterns: [] }))).toBeNull()
  })
})
