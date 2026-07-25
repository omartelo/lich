import type { PanelEdge } from "@/lib/panel-width"
import type { PanelWidth } from "@/lib/use-panel-width"
import { cn } from "@/lib/utils"

interface ResizeHandleProps {
  /** Which edge of the panel it sits on — the same one usePanelWidth drags. */
  edge: PanelEdge
  label: string
  handleProps: PanelWidth["handleProps"]
}

// The drag strip along a resizable panel's edge: a 6px hit area, invisible
// until hovered. The panel must be `relative` for it to land on the edge.
export function ResizeHandle({ edge, label, handleProps }: ResizeHandleProps) {
  return (
    <div
      role="separator"
      aria-orientation="vertical"
      aria-label={label}
      {...handleProps}
      className={cn(
        "absolute top-0 h-full w-1.5 cursor-col-resize touch-none transition-colors hover:bg-accent",
        edge === "right" ? "right-0" : "left-0",
      )}
    />
  )
}
