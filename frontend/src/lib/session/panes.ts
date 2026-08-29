// Splitting the terminal area in two. The whole model is one sentence: a pane
// is a viewport onto a card that already exists, never a session it owns.
//
// That is what makes this unlike a terminal emulator's split, where a pane *is*
// a shell and closing it kills one. lich already has as many sessions as the
// user wants and the sidebar is the multiplexer; the only thing a split buys is
// seeing two of them at once. So the second pane holds a second session id and
// nothing else — closing it closes no session, and every session it can show is
// already a card.
//
// The focused pane is the project's active session, not a third piece of state.
// Switching panes swaps which id is active and which is beside, so the footer,
// the dock, the sidebar highlight and every card shortcut keep following
// activeId exactly as they did before this existed.
//
// Which is why the stored id carries the *side* it sits on. Without it the lanes
// would be read off the swap — the active session drawing left, the beside one
// right — and focusing the right pane would fling its terminal across the stage
// to take the left lane, with the other one sliding under the cursor to replace
// it. The side is what holds both panes still while only the cursor moves.
//
// This file is the pure half — the reconciliation and the drag arithmetic. The
// stored side is panes-store.ts.
import type { Session } from "./sessions"

/** How narrow either pane may be dragged. An agent TUI wants ~80 columns and
 * the stage is what is left after the sidebar and the dock, so a quarter is
 * already the point where the narrower pane stops being readable. */
export const RATIO_MIN = 0.25
export const RATIO_MAX = 0.75
export const RATIO_DEFAULT = 0.5

/** The beside session and the side it draws on, per project. The ratio is
 * deliberately not per project:
 * how wide the reader likes the second pane is a habit of theirs, the way the
 * dock's source is (dock-prefs.ts), not a property of one checkout. */
export const paneKey = (projectId: string): string => `lich.panes.${projectId}`
export const RATIO_KEY = "lich.panes.ratio"

export function clampRatio(ratio: number): number {
  return Math.min(RATIO_MAX, Math.max(RATIO_MIN, ratio))
}

/** Which half of the stage the beside session draws on; the focused session
 * takes the other. */
export type Side = "left" | "right"

export interface Beside {
  id: string
  side: Side
}

export const NO_BESIDE: Beside = { id: "", side: "right" }

export const other = (side: Side): Side => (side === "left" ? "right" : "left")

/** Stored as `<id>` or `<id>@left`. A bare id is the right-hand default, which
 * is what "open beside" means before anything has been swapped — and what a
 * value written by a build that predates sides reads as. */
export function formatBeside(beside: Beside): string {
  return beside.side === "left" ? `${beside.id}@left` : beside.id
}

export function parseBeside(raw: string | null): Beside {
  if (!raw) {
    return NO_BESIDE
  }
  const [id, side] = raw.split("@")
  return { id, side: side === "left" ? "left" : "right" }
}

// resolveBeside is the one guard the whole feature needs against a stale id.
//
// The second pane is derived on every read rather than maintained on every
// mutation: a session leaves the workspace through a close, a park, a worktree
// removal and an undone create, and a field kept in step with all four is four
// chances to leave a pane pointing at a dead id. Answering "is it still in the
// list" at read time is right for every one of those paths, including the ones
// added after this.
//
// An id equal to activeId resolves to nothing for the same reason: a swap
// writes both halves and one card can never hold both panes, so the moment they
// agree the split is over rather than showing the same terminal twice.
export function resolveBeside(
  stored: string,
  sessions: readonly Session[],
  activeId: string,
): Beside {
  const beside = parseBeside(stored)
  if (!beside.id || beside.id === activeId) {
    return NO_BESIDE
  }
  return sessions.some((session) => session.id === beside.id) ? beside : NO_BESIDE
}

// besideCandidate picks what the split shortcut opens beside the active session:
// the next card in the project's own order, wrapping to the first. The sidebar's
// order is the one the user arranged, so "the next one" is the one under the
// card they are on rather than an id nobody can predict.
//
// Nothing to answer with — a project of one session — returns "", and the
// shortcut then declines rather than splitting a pane against itself.
export function besideCandidate(sessions: readonly Session[], activeId: string): string {
  if (sessions.length < 2) {
    return ""
  }
  const at = sessions.findIndex((session) => session.id === activeId)
  if (at < 0) {
    return sessions[0].id
  }
  return sessions[(at + 1) % sessions.length].id
}

// dragRatio resolves the split during a seam drag. Pixels in, ratio out: the
// seam moves with the pointer, and the pane it is dragged into shrinks.
// A container with no width yet (a drag that raced a layout) keeps the ratio it
// started at rather than dividing by zero.
export function dragRatio(
  startRatio: number,
  startX: number,
  clientX: number,
  containerWidth: number,
): number {
  if (containerWidth <= 0) {
    return clampRatio(startRatio)
  }
  return clampRatio(startRatio + (clientX - startX) / containerWidth)
}
