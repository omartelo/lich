// Pure width math for drag-resizable panels. State and storage stay in rem;
// pointer deltas arrive in CSS pixels and are converted with the 16px root
// font size Tailwind assumes.

const REM_PX = 16

export interface WidthBounds {
  minRem: number
  maxRem: number
}

export const clampRem = (rem: number, bounds: WidthBounds): number =>
  Math.min(bounds.maxRem, Math.max(bounds.minRem, rem))

// parseStoredWidth turns a raw localStorage value into a usable width,
// falling back to the default on garbage, negatives or missing values.
export function parseStoredWidth(
  raw: string | null,
  bounds: WidthBounds,
  defaultRem: number,
): number {
  const stored = Number(raw)
  return raw !== null && Number.isFinite(stored) && stored > 0
    ? clampRem(stored, bounds)
    : defaultRem
}

export type PanelEdge = "left" | "right"

// dragWidth resolves the width during a drag: a handle on the panel's right
// edge grows with rightward movement, one on the left edge grows leftward.
export function dragWidth(
  startWidthRem: number,
  startX: number,
  clientX: number,
  edge: PanelEdge,
  bounds: WidthBounds,
): number {
  const deltaPx = edge === "right" ? clientX - startX : startX - clientX
  return clampRem(startWidthRem + deltaPx / REM_PX, bounds)
}
