import { useLayoutEffect, useState } from "react"
import type { RefObject } from "react"

// useElementWidth is an element's content width, read before paint on mount and
// followed after. The first read before paint is what lets a diff card that fits
// side by side draw that way at once, instead of unified first and then again.
export function useElementWidth(ref: RefObject<HTMLElement>): number {
  const [width, setWidth] = useState(0)
  useLayoutEffect(() => {
    const element = ref.current
    if (!element) {
      return
    }
    setWidth(Math.round(element.clientWidth))
    const observer = new ResizeObserver(([entry]) => setWidth(Math.round(entry.contentRect.width)))
    observer.observe(element)
    return () => observer.disconnect()
  }, [ref])
  return width
}
