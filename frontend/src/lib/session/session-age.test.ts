import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { formatAge, nextChangeMs, subscribeAge } from "./session-age"

const SECOND = 1_000
const MINUTE = 60_000
const HOUR = 3_600_000

describe("formatAge", () => {
  it("counts seconds below a minute, floored", () => {
    expect(formatAge(0)).toBe("0s")
    expect(formatAge(1_500)).toBe("1s")
    expect(formatAge(59 * SECOND)).toBe("59s")
    expect(formatAge(59 * SECOND + 999)).toBe("59s")
  })

  it("rolls to minutes at a minute, and to hours at an hour", () => {
    expect(formatAge(MINUTE)).toBe("1m")
    expect(formatAge(90 * MINUTE)).toBe("1h")
    expect(formatAge(HOUR - 1)).toBe("59m")
    expect(formatAge(HOUR)).toBe("1h")
    expect(formatAge(25 * HOUR)).toBe("25h")
  })

  it("reads a backwards clock jump as no time at all", () => {
    expect(formatAge(-5_000)).toBe("0s")
  })
})

describe("nextChangeMs", () => {
  it("waits out the rest of the unit it is counting in", () => {
    expect(nextChangeMs(0)).toBe(SECOND)
    expect(nextChangeMs(500)).toBe(500)
    expect(nextChangeMs(59 * SECOND + 500)).toBe(500)
    expect(nextChangeMs(MINUTE)).toBe(MINUTE)
    expect(nextChangeMs(MINUTE + 30 * SECOND)).toBe(30 * SECOND)
    expect(nextChangeMs(HOUR)).toBe(HOUR)
    expect(nextChangeMs(HOUR + 20 * MINUTE)).toBe(40 * MINUTE)
  })
})

describe("subscribeAge", () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(0)
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it("wakes the subscriber when its readout changes, not before", () => {
    const notify = vi.fn()
    const off = subscribeAge(0, notify)
    vi.advanceTimersByTime(SECOND - 1)
    expect(notify).not.toHaveBeenCalled()
    vi.advanceTimersByTime(1)
    expect(notify).toHaveBeenCalledWith(SECOND)
    off()
  })

  it("holds one timer for every subscriber, and none once they are gone", () => {
    const off1 = subscribeAge(0, vi.fn())
    const off2 = subscribeAge(0, vi.fn())
    expect(vi.getTimerCount()).toBe(1)
    off1()
    expect(vi.getTimerCount()).toBe(1)
    off2()
    expect(vi.getTimerCount()).toBe(0)
  })

  it("sleeps a whole minute for a status only it is timing, in minutes", () => {
    vi.setSystemTime(10 * MINUTE)
    const notify = vi.fn()
    const off = subscribeAge(0, notify)
    vi.advanceTimersByTime(MINUTE - 1)
    expect(notify).not.toHaveBeenCalled()
    vi.advanceTimersByTime(1)
    expect(notify).toHaveBeenCalledTimes(1)
    off()
  })

  // The cadence is the earliest change across every subscriber: a card counting
  // seconds pulls the shared timer down, and its neighbour counting minutes is
  // woken along with it rather than running a timer of its own.
  it("ticks at the earliest change among its subscribers", () => {
    vi.setSystemTime(10 * MINUTE)
    const old = vi.fn()
    const fresh = vi.fn()
    const offOld = subscribeAge(0, old)
    const offFresh = subscribeAge(10 * MINUTE, fresh)
    expect(vi.getTimerCount()).toBe(1)
    vi.advanceTimersByTime(SECOND)
    expect(fresh).toHaveBeenCalledTimes(1)
    expect(old).toHaveBeenCalledTimes(1)
    offOld()
    offFresh()
  })
})
