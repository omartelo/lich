import type { PullRequestDetail } from "@/lib/api-types"

// The merge states GitHub reports that it will still merge from. Everything
// else it refuses, and gh's refusal reaches the screen as one flat sentence
// (internal/project/gherror.go) that names none of the reasons below — so the
// button has to answer this itself, before the click.
//
// UNSTABLE is in: a failing or pending check that no rule requires does not
// block a merge on GitHub, and the checks readout already shows it red. Whether
// to merge over it is the reader's call, not the button's.
const MERGEABLE_STATES = new Set(["CLEAN", "UNSTABLE", "HAS_HOOKS"])

// Why GitHub refuses each of the states it will not merge from. Keyed by gh's
// mergeStateStatus. DRAFT is absent because isDraft says it first and says it
// better; DIRTY is absent because its reason names the base branch.
const BLOCKED_REASONS: Record<string, string> = {
  BLOCKED: "GitHub requires a review or a status check that has not passed",
  BEHIND: "Base branch has moved — update this branch first",
  UNKNOWN: "GitHub is still working out whether this can merge",
}

// mergeBlockedReason names why a pull request cannot be merged right now, or
// returns null when it can. The order is the order a reader would ask in: is
// there anything to merge, is it ready to be merged, will GitHub take it.
export function mergeBlockedReason(detail: PullRequestDetail): string | null {
  if (detail.state !== "OPEN") {
    return `Pull request is ${detail.state.toLowerCase()}`
  }
  if (detail.isDraft) {
    return "Pull request is a draft"
  }
  // Older answers, and any state gh adds after this build, carry no
  // mergeStateStatus. Fall back to the coarse field rather than blocking a
  // merge GitHub would have taken.
  if (detail.mergeStateStatus === "") {
    return detail.mergeable === "CONFLICTING" ? `Conflicts with ${detail.baseRefName}` : null
  }
  if (MERGEABLE_STATES.has(detail.mergeStateStatus)) {
    return null
  }
  if (detail.mergeStateStatus === "DIRTY") {
    return `Conflicts with ${detail.baseRefName}`
  }
  return BLOCKED_REASONS[detail.mergeStateStatus] ?? "GitHub will not merge this pull request yet"
}
