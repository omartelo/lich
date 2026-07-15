// Frame statistics for the Chromium spike page. Standalone (not term-perf):
// that module is inert outside dev builds, and the spike runs from the
// production bundle. Same vocabulary as term-perf's report — stalls are rAF
// gaps >33ms — so numbers compare across the two environments.

export interface FrameStats {
  frames: number
  stalls: number
  worstMs: number
}

export interface FrameAccumulator {
  record(gapMs: number): void
  flush(): FrameStats
}

const STALL_THRESHOLD_MS = 33

export function makeFrameAccumulator(): FrameAccumulator {
  let frames = 0
  let stalls = 0
  let worstMs = 0
  return {
    record(gapMs: number) {
      frames++
      worstMs = Math.max(worstMs, gapMs)
      if (gapMs > STALL_THRESHOLD_MS) {
        stalls++
      }
    },
    flush(): FrameStats {
      const snapshot = { frames, stalls, worstMs }
      frames = 0
      stalls = 0
      worstMs = 0
      return snapshot
    },
  }
}

export function formatStats(stats: FrameStats): string {
  return `[spike] fps=${stats.frames} stalls=${stats.stalls} worst=${stats.worstMs.toFixed(0)}ms`
}

/**
 * Measures rAF gaps and reports one formatted line per second. Returns a stop
 * function. Browser-only (rAF loop); the accumulator/format above carry the
 * testable logic.
 */
export function startFrameStats(report: (line: string) => void): () => void {
  const acc = makeFrameAccumulator()
  let last = performance.now()
  let rafId = 0
  const tick = (now: number) => {
    acc.record(now - last)
    last = now
    rafId = requestAnimationFrame(tick)
  }
  rafId = requestAnimationFrame(tick)
  const timer = window.setInterval(() => report(formatStats(acc.flush())), 1000)
  return () => {
    cancelAnimationFrame(rafId)
    window.clearInterval(timer)
  }
}
