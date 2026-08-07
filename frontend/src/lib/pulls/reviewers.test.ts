import { describe, expect, it } from "vitest"
import type { PullRequestReviewer, ReviewCandidate } from "@/lib/api-types"
import { pendingReviewerLogins, pickableReviewers } from "./reviewers"

const candidate = (login: string, name = ""): ReviewCandidate => ({ login, name })
const reviewer = (login: string, state = "", isTeam = false): PullRequestReviewer => ({
  login,
  state,
  isTeam,
})

describe("pickableReviewers", () => {
  it("sorts by login", () => {
    const picked = pickableReviewers([candidate("cai"), candidate("ana"), candidate("bo")], "zed")
    expect(picked.map((c) => c.login)).toEqual(["ana", "bo", "cai"])
  })

  // GitHub refuses a review request addressed to the author, so offering them
  // is offering a click that can only fail.
  it("drops the pull request's own author", () => {
    const picked = pickableReviewers([candidate("ana"), candidate("bo")], "ana")
    expect(picked.map((c) => c.login)).toEqual(["bo"])
  })

  it("has nothing to offer before the list is read", () => {
    expect(pickableReviewers(null, "ana")).toEqual([])
  })
})

describe("pendingReviewerLogins", () => {
  it("ticks only who still owes a review", () => {
    const pending = pendingReviewerLogins([
      reviewer("ana", "APPROVED"),
      reviewer("bo"),
      reviewer("platform", "", true),
    ])
    expect([...pending].sort()).toEqual(["bo", "platform"])
  })

  it("is empty with no roster", () => {
    expect(pendingReviewerLogins(null).size).toBe(0)
  })
})
