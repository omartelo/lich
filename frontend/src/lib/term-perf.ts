// Dev-only diagnostics for the terminal paint pipeline: per-second console
// report of event/decode/write cost and main-thread rAF stalls. Inert in
// production builds — every entry point is guarded by import.meta.env.DEV, so
// the reporter never starts and the bundler drops the dead branches.

const stats = {
  events: 0,
  bytes: 0,
  decodeMs: 0,
  writeMs: 0,
  stalls: 0,
  worstMs: 0,
  rafMs: 0,
  rafN: 0,
  rafWorstMs: 0,
}
let started = false

// wrapRaf measures every requestAnimationFrame callback so rAF work (xterm's
// renderer, scrollbar fades) can be separated from engine work (paint, GC,
// IPC dispatch) inside a stall: raf≈stall means the callback is guilty,
// raf≪stall means the engine is. Loops that started before the wrap pick it
// up on their next self-rescheduled frame.
function wrapRaf(): void {
  const original = window.requestAnimationFrame.bind(window)
  window.requestAnimationFrame = (callback) =>
    original((time) => {
      const t0 = performance.now()
      callback(time)
      const dt = performance.now() - t0
      stats.rafMs += dt
      stats.rafN++
      stats.rafWorstMs = Math.max(stats.rafWorstMs, dt)
    })
}

// recordChunk logs one data event's cost; the first call starts the reporter.
// For xterm, writeMs includes queue wait + parse (its write is asynchronous).
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
  wrapRaf()

  // rAF gap watcher: frames >33ms are main-thread stalls not accounted for by
  // decode/write — eval of the event payload, terminal paint, GC, React.
  // Stall timestamps (ms, absolute) expose the cadence: ~3000ms spacing points
  // at the git poll, irregular spacing at GC or one-off work.
  let last = performance.now()
  let stallAt: number[] = []
  const tick = (now: number) => {
    const dt = now - last
    last = now
    if (dt > 33) {
      stats.stalls++
      stats.worstMs = Math.max(stats.worstMs, dt)
      if (stallAt.length < 8) {
        stallAt.push(Math.round(now))
      }
    }
    requestAnimationFrame(tick)
  }
  requestAnimationFrame(tick)

  setInterval(() => {
    if (stats.events === 0 && stats.stalls === 0) {
      return
    }
    // eslint-disable-next-line no-console
    console.log(
      `[term-perf] ev/s=${stats.events} KB/s=${(stats.bytes / 1024).toFixed(0)} ` +
        `decode=${stats.decodeMs.toFixed(1)}ms write=${stats.writeMs.toFixed(1)}ms ` +
        `raf=${stats.rafMs.toFixed(0)}ms rafN=${stats.rafN} ` +
        `rafWorst=${stats.rafWorstMs.toFixed(0)}ms ` +
        `stalls=${stats.stalls} worst=${stats.worstMs.toFixed(0)}ms` +
        (stallAt.length > 0 ? ` at=${stallAt.join(",")}` : ""),
    )
    stallAt = []
    stats.events = 0
    stats.bytes = 0
    stats.decodeMs = 0
    stats.writeMs = 0
    stats.stalls = 0
    stats.worstMs = 0
    stats.rafMs = 0
    stats.rafN = 0
    stats.rafWorstMs = 0
  }, 1000)
}
