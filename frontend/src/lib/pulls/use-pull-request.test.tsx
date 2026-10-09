// @vitest-environment jsdom
//
// "No pull request" is an answer a card acts on: it offers to open one. So the
// hook has to keep it apart from "not answered yet", or the offer flashes on a
// branch whose PR is still being looked up.
import { mountBudget } from "@/test/render-budget"
import { createElement, useLayoutEffect, useState } from "react"
import { beforeEach, describe, expect, it, vi } from "vitest"
import type { PullRequest } from "@/lib/api-types"
import { usePullRequest } from "./use-pull-request"

const lookup = vi.fn()
vi.mock("@/lib/rpc", () => ({
  ProjectService: {
    PullRequest: (path: string) => lookup(path),
  },
}))

type Answer = PullRequest | null | undefined

interface Checkout {
  branch: string
  head: string
}

// The card's git poll moving under it: the test swaps the checkout the badge
// reads through this setter, the way a commit or a branch switch would.
let moveTo: (next: Checkout) => void = () => {}

function badge(seen: Answer[], path: string, branch: string, head: string) {
  function Badge() {
    const [checkout, setCheckout] = useState({ branch, head })
    moveTo = setCheckout
    const pr = usePullRequest(path, checkout.branch, checkout.head)
    useLayoutEffect(() => {
      seen.push(pr)
    })
    return null
  }
  return createElement(Badge)
}

beforeEach(() => {
  lookup.mockReset()
})

describe("usePullRequest", () => {
  it("is unanswered, not 'no pull request', while gh is still looking", async () => {
    lookup.mockReturnValue(new Promise(() => {}))
    const seen: Answer[] = []
    await mountBudget(badge(seen, "/pending", "feat", "sha1"))

    expect(seen.every((answer) => answer === undefined)).toBe(true)
  })

  it("answers null for a branch with no pull request", async () => {
    lookup.mockResolvedValue(null)
    const seen: Answer[] = []
    const mounted = await mountBudget(badge(seen, "/none", "feat", "sha1"))
    await mounted.act(() => {})

    expect(seen[seen.length - 1]).toBeNull()
  })

  it("keeps a branch's answer across a new commit", async () => {
    lookup.mockResolvedValue(null)
    const seen: Answer[] = []
    const mounted = await mountBudget(badge(seen, "/commit", "feat", "sha1"))
    await mounted.act(() => {})
    lookup.mockReturnValue(new Promise(() => {}))
    seen.length = 0

    await mounted.act(() => moveTo({ branch: "feat", head: "sha2" }))

    expect(seen.length).toBeGreaterThan(0)
    expect(seen.every((answer) => answer === null)).toBe(true)
  })

  it("forgets the answer when the branch changes", async () => {
    lookup.mockResolvedValue(null)
    const seen: Answer[] = []
    const mounted = await mountBudget(badge(seen, "/switch", "feat", "sha1"))
    await mounted.act(() => {})
    lookup.mockReturnValue(new Promise(() => {}))
    seen.length = 0

    await mounted.act(() => moveTo({ branch: "other", head: "sha3" }))

    expect(seen[seen.length - 1]).toBeUndefined()
  })
})
