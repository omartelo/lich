import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { lookupPullRequest } from "./pull-request-lookup"

// The RPC is the boundary here; stub it so the test asserts how many gh calls
// the lookup actually makes. Each test uses its own checkout path, so the
// module-level share map never carries state across tests.
const pullRequest = vi.fn()
vi.mock("./rpc", () => ({
  ProjectService: {
    PullRequest: (path: string) => pullRequest(path),
  },
}))

const pr = { number: 7, url: "https://github.com/o/l/pull/7", state: "OPEN" }

beforeEach(() => {
  vi.useFakeTimers()
  pullRequest.mockReset()
  pullRequest.mockResolvedValue(pr)
})

afterEach(() => {
  vi.useRealTimers()
})

describe("pull-request-lookup", () => {
  it("collapses a burst of callers into one gh call", async () => {
    const results = await Promise.all([
      lookupPullRequest("/a", "main", "sha1"),
      lookupPullRequest("/a", "main", "sha1"),
      lookupPullRequest("/a", "main", "sha1"),
    ])
    expect(pullRequest).toHaveBeenCalledTimes(1)
    expect(results).toEqual([pr, pr, pr])
  })

  it("shares a slow call however long it takes", async () => {
    let release = (_: typeof pr | null) => {}
    pullRequest.mockReturnValueOnce(new Promise((resolve) => (release = resolve)))
    const first = lookupPullRequest("/b", "main", "sha1")
    // Well past the share window, but the first call has not answered yet.
    vi.setSystemTime(Date.now() + 60_000)
    const second = lookupPullRequest("/b", "main", "sha1")
    release(pr)
    expect(await first).toEqual(pr)
    expect(await second).toEqual(pr)
    expect(pullRequest).toHaveBeenCalledTimes(1)
  })

  it("asks gh again once the answer goes stale", async () => {
    await lookupPullRequest("/c", "main", "sha1")
    vi.setSystemTime(Date.now() + 60_000)
    await lookupPullRequest("/c", "main", "sha1")
    expect(pullRequest).toHaveBeenCalledTimes(2)
  })

  it("does not share across branches", async () => {
    await Promise.all([
      lookupPullRequest("/d", "main", "sha1"),
      lookupPullRequest("/d", "feature", "sha1"),
    ])
    expect(pullRequest).toHaveBeenCalledTimes(2)
  })

  // A commit from the session next door is what opens (or closes) a PR, so a
  // moved HEAD must reach gh rather than replaying the answer for the old one.
  it("asks again when the head commit moves", async () => {
    await lookupPullRequest("/f", "main", "sha1")
    await lookupPullRequest("/f", "main", "sha2")
    expect(pullRequest).toHaveBeenCalledTimes(2)
  })

  it("resolves a failed lookup to no pull request", async () => {
    pullRequest.mockRejectedValueOnce(new Error("gh: not found"))
    await expect(lookupPullRequest("/e", "main", "sha1")).resolves.toBeNull()
  })
})
