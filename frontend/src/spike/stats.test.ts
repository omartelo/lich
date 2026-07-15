import {describe, expect, it} from "vitest"
import {formatStats, makeFrameAccumulator} from "./stats"

describe("makeFrameAccumulator", () => {
  it("counts frames, stalls (>33ms) and the worst gap", () => {
    const acc = makeFrameAccumulator()
    acc.record(16)
    acc.record(17)
    acc.record(50)
    acc.record(120)
    expect(acc.flush()).toEqual({frames: 4, stalls: 2, worstMs: 120})
  })

  it("does not count a 33ms gap as a stall", () => {
    const acc = makeFrameAccumulator()
    acc.record(33)
    expect(acc.flush().stalls).toBe(0)
  })

  it("resets on flush", () => {
    const acc = makeFrameAccumulator()
    acc.record(100)
    acc.flush()
    expect(acc.flush()).toEqual({frames: 0, stalls: 0, worstMs: 0})
  })
})

describe("formatStats", () => {
  it("renders the one-line report", () => {
    expect(formatStats({frames: 59, stalls: 2, worstMs: 47.6})).toBe(
      "[spike] fps=59 stalls=2 worst=48ms",
    )
  })
})
