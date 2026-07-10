// ghostty-web's rAF loop calls renderer.render() every frame, and a scrolled
// viewport (viewportY > 0) forces EVERY row into the dirty set — so sitting
// still reading scrollback repaints the full screen ~30fps forever (~750ms/s
// of paint for zero change). On top of that, scrollback rows come from
// getScrollbackLine, which does one WASM call and allocates ~cols fresh cell
// objects per row per frame. Two patches:
//
// - patchScrollGate: skip render() while scrolled when nothing changed
//   (same viewportY/opacity/theme, no WASM dirty state, no pending selection
//   or hover repaint). Static reading drops to ~0 paints.
// - patchScrollbackCache: memoize getScrollbackLine rows. History is
//   immutable until the terminal writes (buffer append/eviction shifts
//   indices) or resizes (reflow), so both are wrapped to invalidate.
//
// Touches ghostty-web privates (render args, hovered*/previousHovered*,
// theme, selectionManager, getScrollbackLine, write, resize) — revalidate
// when bumping the pinned 0.4.0. Ghostty.clear() writes via wasmTerm.write
// and would bypass the cache invalidation, but lich never calls it.

interface GateBuffer {
  isDirty(): boolean
}

interface GateRenderer {
  theme: object
  hoveredHyperlinkId: number | null
  previousHoveredHyperlinkId: number | null
  hoveredLinkRange: unknown
  previousHoveredLinkRange: unknown
  selectionManager?: { getDirtySelectionRows(): Set<number> } | null
  render(
    buffer: GateBuffer,
    force?: boolean,
    viewportY?: number,
    provider?: unknown,
    opacity?: number,
  ): void
}

export function patchScrollGate(renderer: unknown): void {
  const target = renderer as GateRenderer
  const original = target.render.bind(target)
  let lastY = -1
  let lastOpacity = -1
  let lastTheme: object | null = null

  target.render = (buffer, force = false, viewportY = 0, provider?, opacity = 1) => {
    if (viewportY > 0 && !force) {
      const hoverSynced =
        target.hoveredHyperlinkId === target.previousHoveredHyperlinkId &&
        JSON.stringify(target.hoveredLinkRange) ===
          JSON.stringify(target.previousHoveredLinkRange)
      const selectionClean =
        (target.selectionManager?.getDirtySelectionRows().size ?? 0) === 0
      if (
        viewportY === lastY &&
        opacity === lastOpacity &&
        target.theme === lastTheme &&
        hoverSynced &&
        selectionClean &&
        !buffer.isDirty()
      ) {
        return
      }
    }
    lastY = viewportY
    lastOpacity = opacity
    lastTheme = target.theme
    original(buffer, force, viewportY, provider, opacity)
  }
}

const SCROLLBACK_CACHE_MAX = 256

interface CacheTerm {
  getScrollbackLine(index: number): object[] | null
  write(data: unknown): void
  resize(cols: number, rows: number): void
}

export function patchScrollbackCache(term: unknown): void {
  const target = term as CacheTerm
  const originalGet = target.getScrollbackLine.bind(target)
  const originalWrite = target.write.bind(target)
  const originalResize = target.resize.bind(target)
  const cache = new Map<number, object[]>()

  target.getScrollbackLine = (index) => {
    const hit = cache.get(index)
    if (hit) {
      return hit
    }
    const row = originalGet(index)
    if (row) {
      cache.set(index, row)
      if (cache.size > SCROLLBACK_CACHE_MAX) {
        cache.delete(cache.keys().next().value as number)
      }
    }
    return row
  }
  target.write = (data) => {
    cache.clear()
    originalWrite(data)
  }
  target.resize = (cols, rows) => {
    cache.clear()
    originalResize(cols, rows)
  }
}
