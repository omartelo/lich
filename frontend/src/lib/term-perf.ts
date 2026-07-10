// Dev-only diagnostics for the terminal paint pipeline: per-second console
// report of event/decode/write cost, ghostty render time, glyph-atlas cache
// misses and main-thread rAF stalls. Inert in production builds — every entry
// point is guarded by import.meta.env.DEV, so the reporter never starts and
// the bundler drops the dead branches.

const stats = {
  events: 0,
  bytes: 0,
  decodeMs: 0,
  writeMs: 0,
  stalls: 0,
  worstMs: 0,
  renders: 0,
  renderMs: 0,
  worstRenderMs: 0,
  sprites: 0,
}
let started = false

// instrumentRender wraps a ghostty renderer's render() so paint cost shows up
// in the per-second report — it runs inside ghostty's own rAF, invisible to
// the decode/write timings.
export function instrumentRender(renderer: object): void {
  if (!import.meta.env.DEV) {
    return
  }
  start()
  const target = renderer as { render: (...args: unknown[]) => void }
  const original = target.render.bind(target)
  target.render = (...args: unknown[]) => {
    const t0 = performance.now()
    original(...args)
    const dt = performance.now() - t0
    stats.renders++
    stats.renderMs += dt
    stats.worstRenderMs = Math.max(stats.worstRenderMs, dt)
  }
}

// countingCanvasFactory wraps the glyph-atlas sprite factory so cache misses
// (= sprite creations) show up in the report. High sprites/s means the atlas
// is thrashing (e.g. a TUI animating colors every frame).
export function countingCanvasFactory(): () => HTMLCanvasElement {
  if (!import.meta.env.DEV) {
    return () => document.createElement("canvas")
  }
  return () => {
    stats.sprites++
    return document.createElement("canvas")
  }
}

// recordChunk logs one data event's cost; the first call starts the reporter.
export function recordChunk(decodeMs: number, writeMs: number, bytes: number): void {
  if (!import.meta.env.DEV) {
    return
  }
  stats.events++
  stats.bytes += bytes
  stats.decodeMs += decodeMs
  stats.writeMs += writeMs
  start()
}

function start(): void {
  if (started) {
    return
  }
  started = true

  // rAF gap watcher: frames >33ms are main-thread stalls not accounted for by
  // decode/write — eval of the event payload, ghostty paint, GC, React.
  let last = performance.now()
  const tick = (now: number) => {
    const dt = now - last
    last = now
    if (dt > 33) {
      stats.stalls++
      stats.worstMs = Math.max(stats.worstMs, dt)
    }
    requestAnimationFrame(tick)
  }
  requestAnimationFrame(tick)

  setInterval(() => {
    if (stats.events === 0 && stats.stalls === 0 && stats.renders === 0) {
      return
    }
    // eslint-disable-next-line no-console
    console.log(
      `[term-perf] ev/s=${stats.events} KB/s=${(stats.bytes / 1024).toFixed(0)} ` +
        `decode=${stats.decodeMs.toFixed(1)}ms write=${stats.writeMs.toFixed(1)}ms ` +
        `render=${stats.renderMs.toFixed(1)}ms n=${stats.renders} ` +
        `maxRender=${stats.worstRenderMs.toFixed(0)}ms ` +
        `sprites/s=${stats.sprites} ` +
        `stalls=${stats.stalls} worst=${stats.worstMs.toFixed(0)}ms`,
    )
    stats.events = 0
    stats.bytes = 0
    stats.decodeMs = 0
    stats.writeMs = 0
    stats.stalls = 0
    stats.worstMs = 0
    stats.renders = 0
    stats.renderMs = 0
    stats.worstRenderMs = 0
    stats.sprites = 0
  }, 1000)
}
