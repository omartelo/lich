// Splitting the terminal area into a wall of sessions. The whole model is one
// sentence: a pane is a viewport onto a card that already exists, never a
// session it owns.
//
// That is what makes this unlike a terminal emulator's split, where a pane *is*
// a shell and closing it kills one. lich already has as many sessions as the
// user wants and the sidebar is the multiplexer; the only thing the stage buys
// is seeing several of them at once. So a cell holds a session id and nothing
// else — closing it closes no session, and every session it can show is already
// a card.
//
// The stage is an ordered list *including* the focused session, and the grid is
// computed from how many cells there are and how wide the stage is. Two things
// fall out of that, and both are the reason it is a list rather than a tree:
//
//   - a cell's place is its index, so moving the cursor between panes reorders
//     nothing and no terminal ever slides out from under it;
//   - the focused cell always draws activeId, so a session activated from
//     anywhere — the sidebar, the palette, a link in another session's output,
//     an MCP call — lands in the cell the user was looking at, and none of those
//     paths has to know the stage exists.
//
// This file is the pure half: the reconciliation, the grid and the drag
// arithmetic. The stored side is panes-store.ts.
import type { Session } from "./sessions"

// The narrowest a pane may be laid out before the grid takes another row, and
// the shortest before the stage refuses to add one at all. Width is the binding
// one: an agent TUI wraps its own output, and around fifty columns is where that
// wrapping stops being readable rather than merely tight. Height only has to
// keep a pane from becoming a caption.
export const MIN_PANE_WIDTH = 420
export const MIN_PANE_HEIGHT = 160

/** The smallest share of an axis one row or column may be dragged down to. */
export const MIN_TRACK = 0.12

export const stageKey = (projectId: string): string => `lich.panes.${projectId}`
export const focusKey = (projectId: string): string => `lich.panes.${projectId}.focus`
/** Track sizes are a habit rather than a property of one checkout, the way the
 * dock's source is (dock-prefs.ts), so they are not keyed by project. */
export const COLS_KEY = "lich.panes.cols"
export const ROWS_KEY = "lich.panes.rows"

export interface Grid {
  cols: number
  rows: number
}

export interface Stage {
  /** Session ids, in the order they are laid out; always holds the focused one. */
  cells: string[]
  /** Index of the cell drawing the active session. */
  focus: number
}

// grid picks the layout for a number of panes on a stage of this width: the
// fewest rows that still leave every pane readable.
//
// It is the same arithmetic that used to argue for a hard cap of two, applied
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
// enough to read, which is what stops a held-down shortcut from tiling a project
// into slivers. Height is the limit that bites here: the grid answers a narrow
// stage by taking another row, and rows are what run out.
export function fits(count: number, width: number, height: number): boolean {
  const { rows } = grid(count, width)
  return height / rows >= MIN_PANE_HEIGHT
}

// resolveStage reconciles the stored cells against the project as it is now, and
// is the one guard the whole feature needs.
//
// The cells are derived on every read rather than maintained on every mutation:
// a session leaves the workspace through a close, a park, a worktree removal and
// an undone create, and a list kept in step with all four is four chances to
// leave a pane pointing at a dead id. Answering "is it still in the list" at
// read time is right for every one of those paths, including the ones added
// after this.
//
// Then the focused cell is made to agree with activeId — by moving the focus
// when that session is already on the stage, and by replacing the focused cell's
// occupant when it is not. That second case is what lets a card selected
// anywhere in the app land in the pane the user was looking at, with the rest of
// the wall untouched.
export function resolveStage(
  stored: readonly string[],
  storedFocus: number,
  sessions: readonly Session[],
  activeId: string,
): Stage {
  const live = new Set(sessions.map((session) => session.id))
  const cells: string[] = []
  for (const id of stored) {
    if (live.has(id) && !cells.includes(id)) {
      cells.push(id)
    }
  }
  if (!activeId) {
    return { cells, focus: 0 }
  }
  if (cells.length === 0) {
    return { cells: [activeId], focus: 0 }
  }
  const at = cells.indexOf(activeId)
  if (at >= 0) {
    return { cells, focus: at }
  }
  const focus = Math.min(Math.max(storedFocus, 0), cells.length - 1)
  const replaced = [...cells]
  replaced[focus] = activeId
  return { cells: replaced, focus }
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

// nextCandidate picks what the split shortcut adds: the first card in the
// project's own order that is not already on the stage. The sidebar's order is
// the one the user arranged, so what arrives is the one they can predict.
export function nextCandidate(sessions: readonly Session[], cells: readonly string[]): string {
  return sessions.find((session) => !cells.includes(session.id))?.id ?? ""
}

/** Swap two cells — what a pane dropped on another one does. */
export function swapCells(cells: readonly string[], from: number, to: number): string[] {
  if (from === to || from < 0 || to < 0 || from >= cells.length || to >= cells.length) {
    return [...cells]
  }
  const next = [...cells]
  next[from] = cells[to]
  next[to] = cells[from]
  return next
}
