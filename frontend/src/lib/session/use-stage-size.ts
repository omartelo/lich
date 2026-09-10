import { useEffect, useRef, useState } from "react"
import { setStageSize } from "./panes-store"

export interface StageSize {
  width: number
  height: number
}

// The stage's own pixel size, which is what decides the grid: how many panes fit
// across before the layout takes another row, and whether one more would leave
// them too small to read at all.
//
// Measured rather than derived from the window, because the sidebar collapses,
// the dock opens and both come out of the same width — a number computed from
// the viewport would be wrong for most of the states the window is actually in.
export function useStageSize(): [React.MutableRefObject<HTMLDivElement | null>, StageSize] {
  const ref = useRef<HTMLDivElement | null>(null)
  const [size, setSize] = useState<StageSize>({ width: 0, height: 0 })

  useEffect(() => {
    const element = ref.current
    if (!element) {
      return
    }
    const observer = new ResizeObserver(([entry]) => {
      const { width, height } = entry.contentRect
      // Rounded, so a fractional layout change cannot spin the grid between two
      // answers; the thresholds it feeds are hundreds of pixels apart.
      setStageSize(Math.round(width), Math.round(height))
      setSize((current) =>
        current.width === Math.round(width) && current.height === Math.round(height)
          ? current
          : { width: Math.round(width), height: Math.round(height) },
      )
    })
    observer.observe(element)
    return () => observer.disconnect()
  }, [])

  return [ref, size]
}
