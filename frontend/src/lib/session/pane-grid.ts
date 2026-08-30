// The layout arithmetic behind a wall of panes: how many rows and columns a
// number of sessions takes on a stage of a given width, where each cell lands,
// and what a drag on a seam does to the shares either side of it.
//
// Pure, and deliberately knowing nothing about which sessions are in a group —
// panes.ts owns that. The two questions move for entirely different reasons: one
// changes when the window is resized, the other when the user arranges a group.

// The narrowest a pane may be laid out before the grid takes another row, and
// the shortest before the stage refuses to add one at all. Width is the binding
// one: an agent TUI wraps its own output, and around fifty columns is where that
// wrapping stops being readable rather than merely tight. Height only has to
// keep a pane from becoming a caption.
export const MIN_PANE_WIDTH = 420
export const MIN_PANE_HEIGHT = 160

/** The smallest share of an axis one row or column may be dragged down to. */
export const MIN_TRACK = 0.12

export interface Grid {
  cols: number
  rows: number
}

// grid picks the layout for a number of panes on a stage of this width: the
// fewest rows that still leave every pane readable.
//
// It is the same arithmetic that once argued for a hard cap of two, applied
// continuously instead of frozen at one screen: a 3440px ultrawide fits four
// across, so eight panes there land as 4×2, and the same eight on a laptop come
// out 2×4. Nothing to configure, and the answer follows the window rather than a
// number somebody typed once.
export function grid(count: number, width: number): Grid {
  if (count <= 1) {
    return { cols: 1, rows: 1 }
  }
  for (let rows = 1; rows <= count; rows++) {
    const cols = Math.ceil(count / rows)
    if (width / cols >= MIN_PANE_WIDTH) {
      return { cols, rows }
    }
  }
  // Nothing fits side by side — a very narrow window — so stack them and let
  // the panes be as wide as the stage is.
  return { cols: 1, rows: count }
}

// fits reports whether one more pane would still leave every one of them big
// enough to read, which is what stops a held-down shortcut from tiling a group
// into slivers. Height is the limit that bites here: the grid answers a narrow
// stage by taking another row, and rows are what run out.
export function fits(count: number, width: number, height: number): boolean {
  const { rows } = grid(count, width)
  return height / rows >= MIN_PANE_HEIGHT
}

/** Where a cell sits in the grid. Rows fill left to right, in list order. */
export function cellAt(index: number, { cols }: Grid): { col: number; row: number } {
  return { col: index % cols, row: Math.floor(index / cols) }
}

/** How many cells that row actually holds — the last one may be short. */
export function rowLength(row: number, count: number, { cols }: Grid): number {
  return Math.min(cols, count - row * cols)
}

// tracks returns the share of an axis each row or column takes: the stored
// fractions when they still describe this many tracks, and equal shares
// otherwise. A stored vector of the wrong length is not an error — the grid
// reshapes itself whenever a pane is added or the window is resized, and a
// layout the user dragged for four columns says nothing about three.
export function tracks(stored: readonly number[], count: number): number[] {
  if (count <= 0) {
    return []
  }
  const usable =
    stored.length === count &&
    stored.every((value) => Number.isFinite(value) && value >= MIN_TRACK) &&
    Math.abs(stored.reduce((sum, value) => sum + value, 0) - 1) < 0.01
  return usable ? [...stored] : Array.from({ length: count }, () => 1 / count)
}

// rowTracks narrows the column shares to the cells a short last row has, kept
// in proportion so a row of three under a grid of four spreads across the whole
// width instead of leaving a gap where the fourth would have been.
export function rowTracks(cols: readonly number[], length: number): number[] {
  const used = cols.slice(0, length)
  const total = used.reduce((sum, value) => sum + value, 0)
  return total > 0 ? used.map((value) => value / total) : used
}

/** The offset of a track's leading edge, as a share of the axis. */
export function offsetOf(tracks: readonly number[], index: number): number {
  return tracks.slice(0, index).reduce((sum, value) => sum + value, 0)
}

// dragTrack moves the boundary between track `index` and the one after it,
// taking from one and giving to the other so the axis still sums to one. Neither
// may be pushed below MIN_TRACK, which is what keeps a drag from collapsing a
// pane to nothing and stranding the session inside it.
export function dragTrack(
  start: readonly number[],
  index: number,
  delta: number,
  min = MIN_TRACK,
): number[] {
  if (index < 0 || index >= start.length - 1) {
    return [...start]
  }
  const pair = start[index] + start[index + 1]
  const first = Math.min(pair - min, Math.max(min, start[index] + delta))
  const next = [...start]
  next[index] = first
  next[index + 1] = pair - first
  return next
}
