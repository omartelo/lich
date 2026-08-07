import type { PullRequestReviewer, ReviewCandidate } from "@/lib/api-types"

// Who the picker may offer: everyone the repository allows on a review, minus
// the pull request's own author — GitHub refuses a request addressed to them,
// and an item that can only fail is not an item. Sorted by login so the menu
// keeps one order between reads, whatever order GitHub answered in.
export function pickableReviewers(
  candidates: ReviewCandidate[] | null,
  author: string,
): ReviewCandidate[] {
  return (candidates ?? [])
    .filter((c) => c.login !== "" && c.login !== author)
    .sort((a, b) => a.login.localeCompare(b.login))
}

// The logins the picker shows ticked: those still owing an answer. Somebody who
// already reviewed is not ticked — GitHub cleared their request when the review
// landed, so the tick would offer to withdraw nothing, and the empty box is what
// asks them again.
export function pendingReviewerLogins(reviewers: PullRequestReviewer[] | null): Set<string> {
  return new Set((reviewers ?? []).filter((r) => r.state === "").map((r) => r.login))
}
