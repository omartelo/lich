import { useLayoutEffect, useState } from "react"
import type { RefObject } from "react"

// useWidthAtLeast answers whether an element's content box is at least minPx
// wide, or null before the first measure. A yes/no rather than the width: an
// answer that did not change is a state update React drops, so dragging the dock
// re-renders only the elements that crossed the line. Read before paint on mount,
// so a diff card that fits side by side never draws unified first.
export function useWidthAtLeast(ref: RefObject<HTMLElement>, minPx: number): boolean | null {
  const [fits, setFits] = useState<boolean | null>(null)
  useLayoutEffect(() => {
    const element = ref.current
    if (!element) {
      return
    }
    setFits(element.clientWidth >= minPx)
    const observer = new ResizeObserver(([entry]) => setFits(entry.contentRect.width >= minPx))
    observer.observe(element)
    return () => observer.disconnect()
  }, [ref, minPx])
  return fits
}
