// ghostty-web's compat getLine() copies every cell on every call:
// `getViewport().slice(...).map(cell => ({...cell}))` — one fresh object per
// cell, per row, per frame. During a full-screen repaint that is rows × cols
// allocations per frame (~240k objects/s at peak), which is both the 0.4ms/line
// render cost and the food for the trailing GC stalls. The viewport is already
// parsed into a reusable cell pool that is mutated in place (parseCellsIntoPool
// rewrites all 11 cell fields), so handing out references is safe: every
// getLine consumer in 0.4.0 (render loop, selection text, word-at-cell,
// hyperlink hover) reads the cells synchronously and never mutates or retains
// them. This patch replaces getLine with a cached-row view into the pool —
// zero allocation per call. Touches ghostty-web privates (_cols, _rows,
// update, getViewport) — revalidate when bumping the pinned 0.4.0. Not applied
// after Ghostty.reset(), which recreates wasmTerm (lich never calls it); the
// unpatched fallback is just the original slow copy.

interface PooledTerm {
  _cols: number
  _rows: number
  update(): number
  getViewport(): object[]
  getLine(row: number): object[] | null
}

export function patchPooledGetLine(wasmTerm: unknown): void {
  if (!wasmTerm) {
    return
  }
  const target = wasmTerm as PooledTerm
  let cachePool: object[] | null = null
  let cacheCols = -1
  let cacheRows = -1
  let rowCache: object[][] = []

  target.getLine = (row: number) => {
    if (row < 0 || row >= target._rows) {
      return null
    }
    target.update()
    const pool = target.getViewport()
    if (pool !== cachePool || target._cols !== cacheCols || target._rows !== cacheRows) {
      cachePool = pool
      cacheCols = target._cols
      cacheRows = target._rows
      rowCache = []
      for (let y = 0; y < cacheRows; y++) {
        const cells = new Array<object>(cacheCols)
        for (let x = 0; x < cacheCols; x++) {
          cells[x] = pool[y * cacheCols + x]
        }
        rowCache.push(cells)
      }
    }
    return rowCache[row]
  }
}
