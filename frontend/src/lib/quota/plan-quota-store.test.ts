import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import type { QuotaPlan } from "@/lib/api-types"
import { createPlanQuotaStore } from "./plan-quota-store"

const reading = (percent: number): QuotaPlan[] => [
  {
    provider: "claude",
    name: "Claude Code",
    plan: "Max 5x",
    status: "ok",
    windows: [{ label: "Session", seconds: 18000, percent }],
  },
]

describe("createPlanQuotaStore", () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => vi.useRealTimers())

  it("reads once the first subscriber arrives, and shares that reading", async () => {
    const fetch = vi.fn(async () => reading(2))
    const store = createPlanQuotaStore(fetch)

    expect(store.getSnapshot()).toEqual([])
    const a = store.subscribe(() => {})
    const b = store.subscribe(() => {})
    await vi.advanceTimersByTimeAsync(0)

    expect(fetch).toHaveBeenCalledTimes(1)
    expect(store.getSnapshot()).toEqual(reading(2))
    a()
    b()
  })

  it("keeps the same array while the numbers do not change", async () => {
    const store = createPlanQuotaStore(async () => reading(2), 1000)
    const notify = vi.fn()
    const stop = store.subscribe(notify)
    await vi.advanceTimersByTimeAsync(0)
    const first = store.getSnapshot()

    await vi.advanceTimersByTimeAsync(1000)

    expect(store.getSnapshot()).toBe(first)
    expect(notify).toHaveBeenCalledTimes(1)
    stop()
  })

  it("publishes a changed reading to every subscriber", async () => {
    let percent = 2
    const store = createPlanQuotaStore(async () => reading(percent), 1000)
    const notify = vi.fn()
    const stop = store.subscribe(notify)
    await vi.advanceTimersByTimeAsync(0)

    percent = 91
    await vi.advanceTimersByTimeAsync(1000)

    expect(store.getSnapshot()[0].windows?.[0].percent).toBe(91)
    expect(notify).toHaveBeenCalledTimes(2)
    stop()
  })

  it("keeps the last reading when a read fails", async () => {
    let fail = false
    const store = createPlanQuotaStore(async () => {
      if (fail) {
        throw new Error("offline")
      }
      return reading(2)
    }, 1000)
    const stop = store.subscribe(() => {})
    await vi.advanceTimersByTimeAsync(0)

    fail = true
    await vi.advanceTimersByTimeAsync(1000)

    expect(store.getSnapshot()).toEqual(reading(2))
    stop()
  })

  it("keeps the last reading when a read returns nothing", async () => {
    let empty = false
    const store = createPlanQuotaStore(async () => (empty ? null : reading(2)), 1000)
    const stop = store.subscribe(() => {})
    await vi.advanceTimersByTimeAsync(0)

    empty = true
    await vi.advanceTimersByTimeAsync(1000)

    expect(store.getSnapshot()).toEqual(reading(2))
    stop()
  })

  it("stops polling once the last subscriber leaves", async () => {
    const fetch = vi.fn(async () => reading(2))
    const store = createPlanQuotaStore(fetch, 1000)

    const stop = store.subscribe(() => {})
    await vi.advanceTimersByTimeAsync(0)
    stop()
    await vi.advanceTimersByTimeAsync(5000)

    expect(fetch).toHaveBeenCalledTimes(1)
  })

  it("notices a plan whose status changed but whose windows did not", async () => {
    let status: QuotaPlan["status"] = "ok"
    const store = createPlanQuotaStore(
      async () => [{ provider: "codex", name: "Codex", status }],
      1000,
    )
    const notify = vi.fn()
    const stop = store.subscribe(notify)
    await vi.advanceTimersByTimeAsync(0)

    status = "signed-out"
    await vi.advanceTimersByTimeAsync(1000)

    expect(store.getSnapshot()[0].status).toBe("signed-out")
    stop()
  })
})
