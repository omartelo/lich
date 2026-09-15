import { useEffect, useMemo, useRef, useState } from "react"
import type { EditorView } from "@codemirror/view"
import type { RevertLine } from "@/lib/api-types"
import { gapExpanders, type DocLineSelection } from "@/lib/codemirror"
import { firstLines } from "@/lib/codemirror-revert"
import { threadSlots, type SlotElements } from "@/lib/codemirror-threads"
import { revertLinesIn } from "@/lib/git/diff-blocks"
import { formatLineRef, newLineRange, type DiffLine } from "@/lib/git/diff"
import { buildSplitDoc } from "@/lib/git/split-doc"
import type { DiffBodyProps } from "./DiffBody"
import { InjectMenu } from "./InjectMenu"
import { useBlockRevert } from "./useBlockRevert"
import { useDiffComments, useSlotSync } from "./useDiffComments"
import { useDiffEditor } from "./useDiffEditor"

const NO_SLOTS: SlotElements = new Map()

// SplitDiffBody is the side-by-side layout: HEAD's side in one editor, the new
// side in another, row for row (buildSplitDoc). Threads, comment boxes and the
// gap expanders live in the right column, where comments anchor; the left one
// holds an empty gap of the same height under each, so its rows stay level.
export function SplitDiffBody({
  doc,
  path,
  onInject,
  onSessionComment,
  review,
  onExpandGap,
  onRevertLines,
}: DiffBodyProps) {
  const split = useMemo(() => buildSplitDoc(doc), [doc])
  const comments = useDiffComments({ anchors: split.anchors, path, review, onSessionComment })
  const { gapped, slots } = comments
  const [mirrorElements, setMirrorElements] = useState<SlotElements>(NO_SLOTS)
  const mirror = useMemo(() => threadSlots(setMirrorElements), [])
  const revert = useBlockRevert(doc, onRevertLines)
  const { revertable, hover, revertBlock } = revert
  const [revertTargets, setRevertTargets] = useState<RevertLine[]>([])

  const leftExtra = useMemo(() => {
    const layers = gapped ? [mirror.extension] : []
    if (revertable) {
      layers.push(hover.layer({ blocks: split.blocks, onRevert: revertBlock }))
    }
    return layers.length > 0 ? layers : undefined
  }, [gapped, mirror, revertable, hover, split, revertBlock])
  const rightExtra = useMemo(() => {
    const layers = gapped ? [slots.extension] : []
    if (onExpandGap) {
      layers.push(gapExpanders(split.right.lineMeta, split.right.gaps, onExpandGap))
    }
    if (revertable) {
      const buttons = firstLines(split.blocks)
      layers.push(hover.layer({ blocks: split.blocks, buttons, onRevert: revertBlock }))
    }
    return layers.length > 0 ? layers : undefined
  }, [gapped, slots, onExpandGap, revertable, hover, split, revertBlock])

  const left = useDiffEditor(split.left, path, leftExtra, "old")
  const right = useDiffEditor(split.right, path, rightExtra, "new")
  useSlotSync(right.view, gapped, slots, comments.wanted)
  useSlotSync(left.view, gapped, mirror, comments.wanted)
  useMirrorHeights(comments.elements, mirrorElements, left.view)
  const menuRef = useRef<HTMLDivElement>(null)

  // The selection is read from whichever column holds one, the new side first,
  // and mapped through that column's own lines.
  const resolve = (): { selected: DocLineSelection; meta: DiffLine[] } | null => {
    const onRight = right.getSelectedDocLines()
    if (onRight) {
      return { selected: onRight, meta: split.right.lineMeta }
    }
    const onLeft = left.getSelectedDocLines()
    return onLeft ? { selected: onLeft, meta: split.left.lineMeta } : null
  }

  return (
    <>
      <InjectMenu
        path={path}
        containerRef={menuRef}
        lineRef={comments.selection?.lines ?? null}
        onOpenChange={(open) => {
          if (!open) {
            return
          }
          const found = resolve()
          const range = found && newLineRange(found.meta, found.selected.from, found.selected.to)
          comments.setSelection(range ? { lines: formatLineRef(range), range } : null)
          setRevertTargets(
            found ? revertLinesIn(found.meta, found.selected.from, found.selected.to) : [],
          )
        }}
        onInject={onInject}
        onSessionComment={onSessionComment && comments.openComposer("session")}
        onReviewComment={review && comments.openComposer("review")}
        onRevert={revertable ? () => revert.revertLines(revertTargets) : undefined}
        revertCount={revertTargets.length}
      >
        <div className="grid grid-cols-2">
          <div ref={left.containerRef} className="min-w-0" />
          <div ref={right.containerRef} className="min-w-0 border-l border-border/60" />
        </div>
      </InjectMenu>
      {comments.portals}
    </>
  )
}

// useMirrorHeights keeps each empty gap in the left column as tall as the thread
// or comment box beside it, which grows as someone types or a reply lands.
// ResizeObserver is the framework boundary here; the pairing is by slot key.
function useMirrorHeights(
  sources: SlotElements,
  mirrors: SlotElements,
  view: EditorView | null,
): void {
  useEffect(() => {
    const observers: ResizeObserver[] = []
    for (const [key, source] of sources) {
      const target = mirrors.get(key)
      if (!target) {
        continue
      }
      const observer = new ResizeObserver(() => {
        target.style.boxSizing = "border-box"
        target.style.height = `${source.offsetHeight}px`
        view?.requestMeasure()
      })
      observer.observe(source)
      observers.push(observer)
    }
    return () => {
      for (const observer of observers) {
        observer.disconnect()
      }
    }
  }, [sources, mirrors, view])
}
