import { useRef, useState } from "react"
import type { PointerEvent as ReactPointerEvent } from "react"
import { dragWidth, parseStoredWidth, type PanelEdge, type WidthBounds } from "./panel-width"

export interface PanelWidthOptions extends WidthBounds {
  storageKey: string
  defaultRem: number
  /** Which edge of the panel carries the drag handle. */
  edge: PanelEdge
}

export interface PanelWidth {
  /** Current width in rem — apply as style={{width: `${width}rem`}}. */
  width: number
  /** Spread onto the handle element. */
  handleProps: {
    onPointerDown: (event: ReactPointerEvent<HTMLElement>) => void
    onPointerMove: (event: ReactPointerEvent<HTMLElement>) => void
    onPointerUp: (event: ReactPointerEvent<HTMLElement>) => void
  }
}

// usePanelWidth is the shared drag-resize behavior for side panels: pointer
// capture on the handle, width clamped in rem, persisted to localStorage on
// release. Extracted from the session sidebar so every panel resizes the same.
export function usePanelWidth(options: PanelWidthOptions): PanelWidth {
  const { storageKey, defaultRem, edge } = options
  const [width, setWidth] = useState(() =>
    parseStoredWidth(localStorage.getItem(storageKey), options, defaultRem),
  )
  const dragRef = useRef<{ startX: number; startWidth: number } | null>(null)

  const onPointerDown = (event: ReactPointerEvent<HTMLElement>) => {
    event.preventDefault()
    dragRef.current = { startX: event.clientX, startWidth: width }
    event.currentTarget.setPointerCapture(event.pointerId)
  }

  const onPointerMove = (event: ReactPointerEvent<HTMLElement>) => {
    const drag = dragRef.current
    if (!drag) {
      return
    }
    setWidth(dragWidth(drag.startWidth, drag.startX, event.clientX, edge, options))
  }

  const onPointerUp = (event: ReactPointerEvent<HTMLElement>) => {
    const drag = dragRef.current
    if (!drag) {
      return
    }
    const finalWidth = dragWidth(drag.startWidth, drag.startX, event.clientX, edge, options)
    dragRef.current = null
    event.currentTarget.releasePointerCapture(event.pointerId)
    setWidth(finalWidth)
    localStorage.setItem(storageKey, String(finalWidth))
  }

  return { width, handleProps: { onPointerDown, onPointerMove, onPointerUp } }
}
