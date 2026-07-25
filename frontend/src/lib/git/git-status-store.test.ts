import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { createGitStatusStore, type GitStatus } from "./git-status-store"

// A cadence with a wide gap between fast and slow, so a test can tell which
// one a tick used.
const FAST_MS = 1_000
const SLOW_MS = 10_000
const IDLE_TICKS = 3
const CADENCE = { fastMs: FAST_MS, slowMs: SLOW_MS, idleTicks: IDLE_TICKS }

const status = (files: number, head = "abc123"): GitStatus => ({
  branch: "main",
  files,
  added: files,
  deleted: 0,
  head,
})

beforeEach(() => {
  vi.useFakeTimers()
})

afterEach(() => {
  vi.useRealTimers()
})

describe("createGitStatusStore", () => {
  it("shares one poller across subscribers of the same path", async () => {
    const fetch = vi.fn(async () => status(0))
    const store = createGitStatusStore(fetch, CADENCE)
    const a = store.subscribe("/repo", () => {})
    const b = store.subscribe("/repo", () => {})
    await vi.advanceTimersByTimeAsync(FAST_MS)
    // 1 immediate fetch + 1 tick — not doubled per subscriber.
    expect(fetch).toHaveBeenCalledTimes(2)
    a()
    b()
  })

  it("polls each distinct path separately", async () => {
    const fetch = vi.fn(async (path: string) => (path === "/a" ? status(1) : status(2)))
    const store = createGitStatusStore(fetch, CADENCE)
    store.subscribe("/a", () => {})
    store.subscribe("/b", () => {})
    await vi.advanceTimersByTimeAsync(0)
    expect(store.get("/a")?.files).toBe(1)
    expect(store.get("/b")?.files).toBe(2)
  })

  it("keeps object identity and skips notify when status is unchanged", async () => {
    const fetch = vi.fn(async () => status(3))
    const store = createGitStatusStore(fetch, CADENCE)
    const listener = vi.fn()
    store.subscribe("/repo", listener)
    await vi.advanceTimersByTimeAsync(0)
    const first = store.get("/repo")
    expect(listener).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(FAST_MS * 2)
    expect(store.get("/repo")).toBe(first)
    expect(listener).toHaveBeenCalledTimes(1)
  })

  it("notifies when the status changes", async () => {
    let files = 0
    const fetch = vi.fn(async () => status(files))
    const store = createGitStatusStore(fetch, CADENCE)
    const listener = vi.fn()
    store.subscribe("/repo", listener)
    await vi.advanceTimersByTimeAsync(0)
    files = 5
    await vi.advanceTimersByTimeAsync(FAST_MS)
    expect(listener).toHaveBeenCalledTimes(2)
    expect(store.get("/repo")?.files).toBe(5)
  })

  // A commit leaves the counts untouched (a clean tree before and after) but
  // moves HEAD — the signal the pull request badge keys off, so it has to count
  // as a change or the badge never hears about it.
  it("notifies when only the head commit moves", async () => {
    let head = "aaa"
    const fetch = vi.fn(async () => status(0, head))
    const store = createGitStatusStore(fetch, CADENCE)
    const listener = vi.fn()
    store.subscribe("/repo", listener)
    await vi.advanceTimersByTimeAsync(0)
    expect(listener).toHaveBeenCalledTimes(1)
    head = "bbb"
    await vi.advanceTimersByTimeAsync(FAST_MS)
    expect(listener).toHaveBeenCalledTimes(2)
    expect(store.get("/repo")?.head).toBe("bbb")
  })

  // The whole point of the fast interval is that it does not last: a checkout
  // nobody touches must settle back to the cheap cadence.
  it("slows down after idle ticks and speeds up again on a change", async () => {
    let files = 0
    const fetch = vi.fn(async () => status(files))
    const store = createGitStatusStore(fetch, CADENCE)
    store.subscribe("/repo", () => {})
    await vi.advanceTimersByTimeAsync(0)
    expect(fetch).toHaveBeenCalledTimes(1)

    // Every read finds the same thing, so after IDLE_TICKS of them the next
    // one is a slow tick that a fast interval no longer reaches.
    await vi.advanceTimersByTimeAsync(FAST_MS * IDLE_TICKS)
    const quiet = fetch.mock.calls.length
    expect(quiet).toBe(1 + IDLE_TICKS)
    await vi.advanceTimersByTimeAsync(FAST_MS * 2)
    expect(fetch).toHaveBeenCalledTimes(quiet)

    // The slow tick eventually lands, finds a change, and puts the path back on
    // the fast interval — so the next read is one fast tick away, not one slow.
    files = 3
    await vi.advanceTimersByTimeAsync(SLOW_MS)
    expect(fetch.mock.calls.length).toBeGreaterThan(quiet)
    const busy = fetch.mock.calls.length
    await vi.advanceTimersByTimeAsync(FAST_MS)
    expect(fetch).toHaveBeenCalledTimes(busy + 1)
  })

  it("reports null after a failed fetch", async () => {
    let fail = false
    const fetch = vi.fn(async () => (fail ? null : status(1)))
    const store = createGitStatusStore(fetch, CADENCE)
    store.subscribe("/repo", () => {})
    await vi.advanceTimersByTimeAsync(0)
    expect(store.get("/repo")).not.toBeNull()
    fail = true
    await vi.advanceTimersByTimeAsync(FAST_MS)
    expect(store.get("/repo")).toBeNull()
  })

  it("stops polling when the last subscriber leaves", async () => {
    const fetch = vi.fn(async () => status(0))
    const store = createGitStatusStore(fetch, CADENCE)
    const a = store.subscribe("/repo", () => {})
    const b = store.subscribe("/repo", () => {})
    await vi.advanceTimersByTimeAsync(0)
    a()
    await vi.advanceTimersByTimeAsync(FAST_MS)
    expect(fetch).toHaveBeenCalledTimes(2)
    b()
    await vi.advanceTimersByTimeAsync(FAST_MS * 3)
    expect(fetch).toHaveBeenCalledTimes(2)
    expect(store.get("/repo")).toBeNull()
  })

  it("refreshes a subscribed path immediately, ahead of the poll tick", async () => {
    let files = 0
    const fetch = vi.fn(async () => status(files))
    const store = createGitStatusStore(fetch, CADENCE)
    const listener = vi.fn()
    store.subscribe("/repo", listener)
    await vi.advanceTimersByTimeAsync(0)
    expect(fetch).toHaveBeenCalledTimes(1)

    files = 7
    store.refresh("/repo")
    await vi.advanceTimersByTimeAsync(0)
    expect(fetch).toHaveBeenCalledTimes(2)
    expect(store.get("/repo")?.files).toBe(7)
    expect(listener).toHaveBeenCalledTimes(2)
  })

  it("refresh is a no-op for a path no one subscribes to", async () => {
    const fetch = vi.fn(async () => status(0))
    const store = createGitStatusStore(fetch, CADENCE)
    store.refresh("/nobody-watches-this")
    await vi.advanceTimersByTimeAsync(0)
    expect(fetch).not.toHaveBeenCalled()
  })

  it("a stale fetch resolving late cannot overwrite a fresher one", async () => {
    const resolvers: Array<(value: GitStatus | null) => void> = []
    const fetch = vi.fn(() => new Promise<GitStatus | null>((r) => resolvers.push(r)))
    const store = createGitStatusStore(fetch, CADENCE)
    const listener = vi.fn()
    store.subscribe("/repo", listener) // fetch #1 — the slow poll
    store.refresh("/repo") // fetch #2 — the session-touched nudge
    resolvers[1]?.(status(5)) // fresh result lands first
    await vi.advanceTimersByTimeAsync(0)
    expect(store.get("/repo")?.files).toBe(5)

    resolvers[0]?.(status(1)) // the stale poll resolves late
    await vi.advanceTimersByTimeAsync(0)
    expect(store.get("/repo")?.files).toBe(5)
    expect(listener).toHaveBeenCalledTimes(1)
  })

  it("drops an in-flight result after teardown", async () => {
    let resolve: (value: GitStatus | null) => void = () => {}
    const fetch = vi.fn(() => new Promise<GitStatus | null>((r) => (resolve = r)))
    const store = createGitStatusStore(fetch, CADENCE)
    const listener = vi.fn()
    const unsubscribe = store.subscribe("/repo", listener)
    unsubscribe()
    resolve(status(9))
    await vi.advanceTimersByTimeAsync(0)
    expect(listener).not.toHaveBeenCalled()
    expect(store.get("/repo")).toBeNull()
  })
})
