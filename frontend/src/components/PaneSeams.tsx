import { useRef } from "react"
import type { PointerEvent as ReactPointerEvent } from "react"
import { dragTrack, offsetOf } from "@/lib/session/pane-grid"

interface PaneSeamsProps {
  /** Share of the axis each track takes; a seam sits between adjacent pairs. */
  tracks: number[]
  /** Which way the boundary is dragged: columns move left and right. */
  axis: "cols" | "rows"
  /** Size of the stage along the dragged axis, in pixels. */
  extent: number
  /** Where the seams start and how far they reach across the other axis, as
   * shares of it — a column seam only spans the row it divides. */
  from?: number
  span?: number
  /** During the drag, so the panes follow the pointer. */
  onChange: (tracks: number[]) => void
  /** On release, which is when the layout is written down. */
  onCommit: (tracks: number[]) => void
}

// The grab strips between panes. They are drawn over the terminals rather than
// between them because the panes are absolutely positioned layers, not a flex
// row — the seam is a handle at a computed offset, and the hairline the user
// sees is the border on the pane beside it.
//
// Dragging is pointer-based, like every other resize in the app, and the value
// only reaches the store on release: a drag would otherwise put a hundred writes
// through localStorage. The live value belongs to the stage, not to this
// component — the panes have to move with the pointer too.
export function PaneSeams({
  tracks,
  axis,
  extent,
  from = 0,
  span = 1,
  onChange,
  onCommit,
}: PaneSeamsProps) {
  // The tracks as they were when the drag started: every move is measured from
  // that, so a pointer returning to where it began restores the original sizes
  // rather than accumulating rounding.
  const drag = useRef<{ index: number; start: number; tracks: number[] } | null>(null)
  const vertical = axis === "cols"

  const at = (event: ReactPointerEvent<HTMLElement>) => (vertical ? event.clientX : event.clientY)

  const move = (event: ReactPointerEvent<HTMLElement>): number[] | null => {
    const current = drag.current
    if (!current || extent <= 0) {
      return null
    }
    return dragTrack(current.tracks, current.index, (at(event) - current.start) / extent)
  }

  return (
    <>
      {tracks.slice(0, -1).map((_, index) => {
        const offset = offsetOf(tracks, index + 1)
        return (
          <div
            // A boundary is its place in the axis: the seam between the first
            // and second column is that seam, and the tracks either side of it
            // are what a drag changes. Keying it by a value would remount it
            // mid-drag, which is the one thing a handle must not do.
            // biome-ignore lint/suspicious/noArrayIndexKey: the index is the identity here
            key={`${axis}-${index}`}
            role="separator"
            aria-orientation={vertical ? "vertical" : "horizontal"}
            aria-label={vertical ? "Resize the columns" : "Resize the rows"}
            className={
              vertical
                ? "absolute z-10 w-1.5 -translate-x-1/2 cursor-col-resize touch-none transition-colors hover:bg-accent"
                : "absolute z-10 h-1.5 -translate-y-1/2 cursor-row-resize touch-none transition-colors hover:bg-accent"
            }
            style={
              vertical
                ? { left: `${offset * 100}%`, top: `${from * 100}%`, height: `${span * 100}%` }
                : { top: `${offset * 100}%`, left: `${from * 100}%`, width: `${span * 100}%` }
            }
            onPointerDown={(event) => {
              event.preventDefault()
              drag.current = { index, start: at(event), tracks: [...tracks] }
              event.currentTarget.setPointerCapture(event.pointerId)
            }}
            onPointerMove={(event) => {
              const next = move(event)
              if (next) {
                onChange(next)
              }
            }}
            onPointerUp={(event) => {
              const next = move(event)
              drag.current = null
              event.currentTarget.releasePointerCapture(event.pointerId)
              if (next) {
                onCommit(next)
              }
            }}
            onPointerCancel={() => {
              drag.current = null
            }}
          />
        )
      })}
    </>
  )
}
