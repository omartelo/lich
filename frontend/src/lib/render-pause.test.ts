import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { pauseRenderLoop, resumeRenderLoop } from "./render-pause"

const cancelAnimationFrame = vi.fn()

beforeEach(() => {
  vi.stubGlobal("cancelAnimationFrame", cancelAnimationFrame)
})

afterEach(() => {
  vi.unstubAllGlobals()
  cancelAnimationFrame.mockClear()
})

function makeTerminal(overrides: Record<string, unknown> = {}) {
  return {
    animationFrameId: 42 as number | undefined,
    isOpen: true,
    isDisposed: false,
    startRenderLoop: vi.fn(function (this: { animationFrameId?: number }) {
      this.animationFrameId = 7
    }),
    ...overrides,
  }
}

describe("pauseRenderLoop", () => {
  it("cancels the pending frame and clears the handle", () => {
    const term = makeTerminal()
    pauseRenderLoop(term)

    expect(cancelAnimationFrame).toHaveBeenCalledWith(42)
    expect(term.animationFrameId).toBeUndefined()
  })

  it("is idempotent — a second pause cancels nothing", () => {
    const term = makeTerminal()
    pauseRenderLoop(term)
    pauseRenderLoop(term)

    expect(cancelAnimationFrame).toHaveBeenCalledTimes(1)
  })

  it("no-ops when the loop never started", () => {
    const term = makeTerminal({ animationFrameId: undefined })
    pauseRenderLoop(term)

    expect(cancelAnimationFrame).not.toHaveBeenCalled()
  })

  it("fails open on an unexpected terminal shape", () => {
    pauseRenderLoop({ animationFrameId: 42 })

    expect(cancelAnimationFrame).not.toHaveBeenCalled()
  })
})

describe("resumeRenderLoop", () => {
  it("restarts the loop on a paused open terminal", () => {
    const term = makeTerminal({ animationFrameId: undefined })
    resumeRenderLoop(term)

    expect(term.startRenderLoop).toHaveBeenCalledTimes(1)
    expect(term.animationFrameId).toBe(7)
  })

  it("no-ops when the terminal is not open", () => {
    const term = makeTerminal({ animationFrameId: undefined, isOpen: false })
    resumeRenderLoop(term)

    expect(term.startRenderLoop).not.toHaveBeenCalled()
  })

  it("no-ops when the terminal is disposed", () => {
    const term = makeTerminal({ animationFrameId: undefined, isDisposed: true })
    resumeRenderLoop(term)

    expect(term.startRenderLoop).not.toHaveBeenCalled()
  })

  it("no-ops when the loop is already running (no double loop)", () => {
    const term = makeTerminal()
    resumeRenderLoop(term)

    expect(term.startRenderLoop).not.toHaveBeenCalled()
  })

  it("fails open on an unexpected terminal shape", () => {
    expect(() => resumeRenderLoop({ isOpen: true, isDisposed: false })).not.toThrow()
  })
})
