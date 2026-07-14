// ghostty-web's FitAddon subtracts a fixed 15px scrollbar gutter from the width
// before dividing into cells, so the grid never reaches the right edge — a
// permanent band between the terminal and the window. computeGrid divides the
// full container instead, filling edge to edge. Only a sub-cell remainder is
// left (< 1 cell, unavoidable: partial cells can't render), landing on the
// right/bottom since the canvas is top-left anchored. The trade: the overlay
// scrollbar now paints over the last column while scrolled; reserve a gutter
// again only if that ever bothers.

export interface CellSize {
  width: number
  height: number
}

export interface GridSize {
  cols: number
  rows: number
}

// computeGrid returns the whole-cell grid that fills a container of the given
// pixel size. Returns null when either dimension is unusable (zero cell metrics
// before the font loads, or a zero-size container while hidden/unmounted), so
// callers skip the resize rather than proposing a degenerate grid.
export function computeGrid(
  width: number,
  height: number,
  cell: CellSize,
): GridSize | null {
  if (width <= 0 || height <= 0 || cell.width <= 0 || cell.height <= 0) {
    return null
  }
  return {
    cols: Math.max(1, Math.floor(width / cell.width)),
    rows: Math.max(1, Math.floor(height / cell.height)),
  }
}
