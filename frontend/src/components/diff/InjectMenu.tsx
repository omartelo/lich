import type { RefObject } from "react"
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuTrigger,
} from "@/components/ui/context-menu"

interface InjectMenuProps {
  /** Path written into the terminal, relative to the checkout. */
  path: string
  /** Where the CodeMirror view mounts — the menu wraps it as its trigger. */
  containerRef: RefObject<HTMLDivElement>
  /** The current selection as git-style "12-30", or null when there is none. */
  lineRef: string | null
  /** Resolve the selection; fired as the menu opens, not on every change. */
  onOpenChange: (open: boolean) => void
  onInject: (text: string) => void
}

// The right-click menu over a code view — a file's diff, or the read-only
// preview in the dock — that writes a reference into the session's terminal.
// The whole file, or just the lines under the selection.
//
// The isolate wrapper keeps CodeMirror's high-z-index gutter from painting over
// the sticky header of the card above it.
export function InjectMenu({
  path,
  containerRef,
  lineRef,
  onOpenChange,
  onInject,
}: InjectMenuProps) {
  return (
    <ContextMenu onOpenChange={onOpenChange}>
      <ContextMenuTrigger render={<div className="isolate py-1" ref={containerRef} />} />
      <ContextMenuContent>
        <ContextMenuItem onClick={() => onInject(`@${path} `)}>Inject file</ContextMenuItem>
        <ContextMenuItem
          disabled={lineRef === null}
          onClick={() => lineRef && onInject(`${path}:${lineRef} `)}
        >
          {lineRef === null ? "Inject lines" : `Inject lines ${lineRef}`}
        </ContextMenuItem>
      </ContextMenuContent>
    </ContextMenu>
  )
}
