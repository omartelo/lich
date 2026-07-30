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
  /** Start a comment held for the session's next prompt. Absent on a surface
   * that is not a review — the dock's file preview. */
  onSessionComment?: () => void
  /** Start a comment on the pull request itself. Absent wherever there is no
   * pull request behind the diff — the dock's working diff. */
  onReviewComment?: () => void
}

// The right-click menu over a code view — a file's diff, or the read-only
// preview in the dock — that writes a reference into the session's terminal.
// The whole file, or just the lines under the selection.
//
// One selection, and up to three things to do with it: inject it as a
// reference, comment on it for the session's next prompt, or comment on it for
// the pull request. That fork is the reason the two comment items are named for
// where they go rather than both reading "comment": on a pull request's diff
// both are offered, and picking the wrong one is a note that lands where nobody
// looks for it.
//
// The isolate wrapper keeps CodeMirror's high-z-index gutter from painting over
// the sticky header of the card above it.
export function InjectMenu({
  path,
  containerRef,
  lineRef,
  onOpenChange,
  onInject,
  onSessionComment,
  onReviewComment,
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
        {onSessionComment && (
          <ContextMenuItem disabled={lineRef === null} onClick={onSessionComment}>
            {lineRef === null ? "Comment for the session…" : `Comment for the session ${lineRef}…`}
          </ContextMenuItem>
        )}
        {onReviewComment && (
          <ContextMenuItem disabled={lineRef === null} onClick={onReviewComment}>
            {lineRef === null
              ? "Comment on the pull request…"
              : `Comment on the pull request ${lineRef}…`}
          </ContextMenuItem>
        )}
      </ContextMenuContent>
    </ContextMenu>
  )
}
