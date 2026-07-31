import type { BranchRules, CommitPattern, MergeMethod, PullRequestDetail } from "@/lib/api-types"

// Every method lich knows how to ask gh for, in the order the menu offers them.
const EVERY_METHOD: MergeMethod[] = ["squash", "merge", "rebase"]

// allowedMergeMethods narrows the Merge menu to what the base branch accepts. A
// ruleset can pin a repository to squash alone, and nothing in a pull request's
// own fields says so — the menu offered all three and gh refused after the
// click, with a sentence that named no method.
//
// Everything unknown widens rather than narrows: no rules, an unreadable
// answer, a call that failed, or a ruleset naming only methods this build does
// not have. A menu that lost an option lich could have offered is a worse
// failure than one that offers an option GitHub turns down, because only the
// second one tells you what happened.
export function allowedMergeMethods(rules: BranchRules | null): MergeMethod[] {
  const named = rules?.allowedMergeMethods ?? []
  const allowed = EVERY_METHOD.filter((method) => named.includes(method))
  return allowed.length > 0 ? allowed : EVERY_METHOD
}

// The review verdicts that keep a BLOCKED pull request blocked. An approval —
// or a repository that asks for none — leaves BLOCKED meaning only that a rule
// applies to the base branch, which GitHub merges from every day.
const REVIEW_PENDING: Record<string, string> = {
  CHANGES_REQUESTED: "A reviewer asked for changes",
  REVIEW_REQUIRED: "GitHub requires a review that has not been left",
}

// mergeBlockedReason names why GitHub would refuse this merge, or returns null
// when the button should let the click through. gh's own refusal reaches the
// screen as one flat sentence (internal/project/gherror.go) that names no
// reason, so what can be answered before the click is answered here.
//
// What is deliberately *not* answered here is the rest: this names the refusals
// and lets everything else through, rather than allow-listing the states GitHub
// merges from. mergeStateStatus BLOCKED is why. GitHub reports it for any base
// branch carrying a protection rule or a ruleset — it means "a rule applies
// here", not "I will not take this". A ruleset whose commit-message pattern is
// only evaluated against the merge commit leaves an approved pull request with
// green checks and no conflicts sitting at BLOCKED, while GitHub's own button
// merges it. Reading that as a refusal is a dead Merge button on every governed
// repository, which is worse than the toast this replaced: a merge that is
// never tried cannot report why it failed.
//
// The order is the order a reader would ask in: is there anything to merge, is
// it ready, will GitHub take it.
export function mergeBlockedReason(detail: PullRequestDetail): string | null {
  if (detail.state !== "OPEN") {
    return `Pull request is ${detail.state.toLowerCase()}`
  }
  if (detail.isDraft) {
    return "Pull request is a draft"
  }
  // Both fields, because they answer at different times: mergeable is what an
  // older gh reports, mergeStateStatus what a current one does.
  if (detail.mergeStateStatus === "DIRTY" || detail.mergeable === "CONFLICTING") {
    return `Conflicts with ${detail.baseRefName}`
  }
  if (detail.mergeStateStatus === "BEHIND") {
    return "Base branch has moved — update this branch first"
  }
  if (detail.mergeStateStatus === "BLOCKED") {
    return REVIEW_PENDING[detail.reviewDecision] ?? null
  }
  return null
}

// The merge states an administrator override gets past, which is gh's own
// allowsAdminOverride — pass --admin on anything else and gh refuses from the
// client anyway. A conflict is absent on purpose: the override skips a rule, it
// does not compute a merge that has no answer.
const OVERRIDABLE = ["BLOCKED", "BEHIND"]

// canAdminOverride reports whether the Merge menu should offer to bypass the
// rules on the base branch: something has to be in the way that an override
// actually clears, and the account has to hold the privilege.
//
// This is the one place the menu does *not* widen on an unknown. Everything
// else here would rather offer an option GitHub turns down than drop one it
// would have taken, because a refusal at least says what happened — but a
// bypass is read as a permission you have, so offering one that is not there
// makes GitHub's refusal look like a broken button. Absent is the honest shape
// of "not yours"; the backend answers the same way (branchrules.go).
export function canAdminOverride(detail: PullRequestDetail, rules: BranchRules | null): boolean {
  if (!rules?.viewerCanBypass) {
    return false
  }
  if (detail.state !== "OPEN" || detail.isDraft) {
    return false
  }
  return OVERRIDABLE.includes(detail.mergeStateStatus)
}

// GitHub's pattern operators in the words a sentence needs. An operator this
// build has never seen falls back to "match": the pattern beside it still says
// more than no note at all.
const PATTERN_VERBS: Record<string, string> = {
  regex: "match",
  starts_with: "start with",
  ends_with: "end with",
  contains: "contain",
}

function describePattern(rule: CommitPattern): string {
  const verb = PATTERN_VERBS[rule.operator] ?? "match"
  return `${rule.target} must ${rule.negate ? "not " : ""}${verb} ${rule.pattern}`
}

// mergeRuleNote names what governs the base branch when GitHub answers BLOCKED
// and nothing on the pull request accounts for it — the approved, green,
// conflict-free case, where the cause is a rule read against the commit the
// merge would write and no field on screen mentions it.
//
// It is a note, never a verdict: it names the rules that exist, not the one
// that fired. GitHub publishes no "why is this BLOCKED", so the honest answer
// is where to look — and re-running the pattern here would be a guess dressed
// as an answer, in a regex dialect that is not the one GitHub evaluates.
export function mergeRuleNote(detail: PullRequestDetail, rules: BranchRules | null): string | null {
  if (detail.mergeStateStatus !== "BLOCKED") {
    return null
  }
  const patterns = rules?.commitPatterns ?? []
  if (patterns.length === 0) {
    return null
  }
  const clauses = patterns.map(describePattern).join("; ")
  return `A ruleset governs ${detail.baseRefName}: every commit ${clauses}.`
}
