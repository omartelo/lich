// @vitest-environment jsdom
//
// What this hook has to get right is when it stays quiet: naming the conflicting
// files costs a git fetch, and a pull request that merges cleanly must never pay
// for it. That is a count of calls, so the RPC is stubbed and the hook is
// mounted for real.
import { mountBudget } from "@/test/render-budget"
import { StrictMode, createElement, useLayoutEffect } from "react"
import { beforeEach, describe, expect, it, vi } from "vitest"
import { usePullRequestConflicts } from "./use-pull-request-conflicts"

const PR_URL = "https://github.com/owner/repo/pull/7"

const conflicts = vi.fn()
vi.mock("@/lib/rpc", () => ({
  ProjectService: {
    PullRequestConflicts: (path: string, number: number, base: string, url: string) =>
      conflicts(path, number, base, url),
  },
}))

// Records every answer the hook committed, from a layout effect so a render
// React discarded is not counted as one the screen saw.
function panel(seen: string[][], conflicting: boolean, number = 7, base = "main") {
  function Panel() {
    const { files } = usePullRequestConflicts("/repo", number, base, PR_URL, conflicting)
    useLayoutEffect(() => {
      seen.push(files)
    })
    return null
  }
  return createElement(StrictMode, null, createElement(Panel))
}

beforeEach(() => {
  conflicts.mockReset()
  conflicts.mockResolvedValue(["internal/project/pr.go", "CHANGELOG.md"])
})

describe("usePullRequestConflicts", () => {
  it("names the files GitHub would not", async () => {
    const seen: string[][] = []
    const mounted = await mountBudget(panel(seen, true))
    await mounted.act(() => {})

    expect(conflicts).toHaveBeenCalledWith("/repo", 7, "main", PR_URL)
    expect(seen[seen.length - 1]).toEqual(["internal/project/pr.go", "CHANGELOG.md"])
  })

  it("asks nothing of a pull request that merges cleanly", async () => {
    const seen: string[][] = []
    const mounted = await mountBudget(panel(seen, false))
    await mounted.act(() => {})

    expect(conflicts).not.toHaveBeenCalled()
    expect(seen[seen.length - 1]).toEqual([])
  })

  it("asks nothing before the pull request is addressed", async () => {
    const seen: string[][] = []
    const mounted = await mountBudget(panel(seen, true, 0))
    await mounted.act(() => {})

    expect(conflicts).not.toHaveBeenCalled()
    expect(seen[seen.length - 1]).toEqual([])
  })

  it("asks nothing without a base branch to merge into", async () => {
    const seen: string[][] = []
    const mounted = await mountBudget(panel(seen, true, 7, ""))
    await mounted.act(() => {})

    expect(conflicts).not.toHaveBeenCalled()
  })
})
