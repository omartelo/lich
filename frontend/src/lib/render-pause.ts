// ghostty-web runs a continuous requestAnimationFrame render loop per terminal
// (~60fps, started in open(), stopped only by dispose()) with no visibility
// gating — N mounted terminals cost N render passes per frame even when hidden
// and idle. These helpers reach into the terminal's private loop state to stop
// the loop while a terminal is hidden and restart it on show. Writes keep
// updating the WASM buffer and its dirty flags while paused, so resuming
// repaints correctly.

interface PausableTerminal {
  animationFrameId?: number
  isOpen: boolean
  isDisposed: boolean
  startRenderLoop(): void
}

// hasRenderLoop guards against ghostty-web renaming its private internals: when
// the shape no longer matches, both helpers fail open — the terminal keeps
// rendering at 60fps (today's behavior) instead of freezing unresumably.
function hasRenderLoop(term: PausableTerminal): boolean {
  return typeof term.startRenderLoop === "function"
}

/**
 * Stops the terminal's render loop by cancelling the pending animation frame.
 * Idempotent; a terminal whose loop never started (or already paused) is left
 * untouched.
 */
export function pauseRenderLoop(terminal: unknown): void {
  const term = terminal as PausableTerminal
  if (!hasRenderLoop(term) || term.animationFrameId === undefined) {
    return
  }
  cancelAnimationFrame(term.animationFrameId)
  term.animationFrameId = undefined
}

/**
 * Restarts a paused render loop. No-op when the terminal is not open, already
 * disposed, or the loop is still running (startRenderLoop assigns the frame
 * handle synchronously, so a defined handle means a live loop).
 */
export function resumeRenderLoop(terminal: unknown): void {
  const term = terminal as PausableTerminal
  if (!hasRenderLoop(term) || !term.isOpen || term.isDisposed) {
    return
  }
  if (term.animationFrameId !== undefined) {
    return
  }
  term.startRenderLoop()
}
