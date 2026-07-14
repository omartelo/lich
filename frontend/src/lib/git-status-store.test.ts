import {afterEach, beforeEach, describe, expect, it, vi} from "vitest"
import {createGitStatusStore, type GitStatus} from "./git-status-store"

const POLL_MS = 3_000

const status = (files: number): GitStatus => ({
  branch: "main",
  files,
  added: files,
  deleted: 0,
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
    const store = createGitStatusStore(fetch, POLL_MS)
    const a = store.subscribe("/repo", () => {})
    const b = store.subscribe("/repo", () => {})
    await vi.advanceTimersByTimeAsync(POLL_MS)
    // 1 immediate fetch + 1 tick — not doubled per subscriber.
    expect(fetch).toHaveBeenCalledTimes(2)
    a()
    b()
  })

  it("polls each distinct path separately", async () => {
    const fetch = vi.fn(async (path: string) =>
      path === "/a" ? status(1) : status(2),
    )
    const store = createGitStatusStore(fetch, POLL_MS)
    store.subscribe("/a", () => {})
    store.subscribe("/b", () => {})
    await vi.advanceTimersByTimeAsync(0)
    expect(store.get("/a")?.files).toBe(1)
    expect(store.get("/b")?.files).toBe(2)
  })

  it("keeps object identity and skips notify when status is unchanged", async () => {
    const fetch = vi.fn(async () => status(3))
    const store = createGitStatusStore(fetch, POLL_MS)
    const listener = vi.fn()
    store.subscribe("/repo", listener)
    await vi.advanceTimersByTimeAsync(0)
    const first = store.get("/repo")
    expect(listener).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(POLL_MS * 2)
    expect(store.get("/repo")).toBe(first)
    expect(listener).toHaveBeenCalledTimes(1)
  })

  it("notifies when the status changes", async () => {
    let files = 0
    const fetch = vi.fn(async () => status(files))
    const store = createGitStatusStore(fetch, POLL_MS)
    const listener = vi.fn()
    store.subscribe("/repo", listener)
    await vi.advanceTimersByTimeAsync(0)
    files = 5
    await vi.advanceTimersByTimeAsync(POLL_MS)
    expect(listener).toHaveBeenCalledTimes(2)
    expect(store.get("/repo")?.files).toBe(5)
  })

  it("reports null after a failed fetch", async () => {
    let fail = false
    const fetch = vi.fn(async () => (fail ? null : status(1)))
    const store = createGitStatusStore(fetch, POLL_MS)
    store.subscribe("/repo", () => {})
    await vi.advanceTimersByTimeAsync(0)
    expect(store.get("/repo")).not.toBeNull()
    fail = true
    await vi.advanceTimersByTimeAsync(POLL_MS)
    expect(store.get("/repo")).toBeNull()
  })

  it("stops polling when the last subscriber leaves", async () => {
    const fetch = vi.fn(async () => status(0))
    const store = createGitStatusStore(fetch, POLL_MS)
    const a = store.subscribe("/repo", () => {})
    const b = store.subscribe("/repo", () => {})
    await vi.advanceTimersByTimeAsync(0)
    a()
    await vi.advanceTimersByTimeAsync(POLL_MS)
    expect(fetch).toHaveBeenCalledTimes(2)
    b()
    await vi.advanceTimersByTimeAsync(POLL_MS * 3)
    expect(fetch).toHaveBeenCalledTimes(2)
    expect(store.get("/repo")).toBeNull()
  })

  it("refreshes a subscribed path immediately, ahead of the poll tick", async () => {
    let files = 0
    const fetch = vi.fn(async () => status(files))
    const store = createGitStatusStore(fetch, POLL_MS)
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
    const store = createGitStatusStore(fetch, POLL_MS)
    store.refresh("/nobody-watches-this")
    await vi.advanceTimersByTimeAsync(0)
    expect(fetch).not.toHaveBeenCalled()
  })

  it("drops an in-flight result after teardown", async () => {
    let resolve: (value: GitStatus | null) => void = () => {}
    const fetch = vi.fn(
      () => new Promise<GitStatus | null>((r) => (resolve = r)),
    )
    const store = createGitStatusStore(fetch, POLL_MS)
    const listener = vi.fn()
    const unsubscribe = store.subscribe("/repo", listener)
    unsubscribe()
    resolve(status(9))
    await vi.advanceTimersByTimeAsync(0)
    expect(listener).not.toHaveBeenCalled()
    expect(store.get("/repo")).toBeNull()
  })
})
