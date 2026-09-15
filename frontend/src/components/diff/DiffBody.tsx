import { useMemo, useState } from "react"
import type { RevertLine } from "@/lib/api-types"
import { gapExpanders } from "@/lib/codemirror"
import { firstLines } from "@/lib/codemirror-revert"
import { blockOfLine, revertLinesIn } from "@/lib/git/diff-blocks"
import { formatLineRef, newLineRange, type DiffGap, type FileDoc } from "@/lib/git/diff"
import { InjectMenu } from "./InjectMenu"
import type { DiffReview } from "./ReviewSlots"
import { useBlockRevert } from "./useBlockRevert"
import { useDiffComments, useSlotSync } from "./useDiffComments"
import { useDiffEditor } from "./useDiffEditor"

export interface DiffBodyProps {
  doc: FileDoc
  path: string
  onInject: (text: string) => void
  onSessionComment?: (path: string, lines: string, text: string) => void
  review?: DiffReview
  /** Pull one gap's lines in. Absent = the diff draws no expanders. */
  onExpandGap?: (gap: DiffGap) => void
  /** Put changed lines back to HEAD. Absent = the diff offers no revert. */
  onRevertLines?: (lines: RevertLine[]) => void
}

// DiffBody is the unified layout, and exists as its own component so collapsing
// the file unmounts it, destroying the CodeMirror view instead of keeping it
// alive off-screen.
//
// A diff's document is not the file: the selection lands on doc lines, which
// newLineRange maps back to the line numbers an inject writes, a session comment
// anchors to, and GitHub files a review comment against.
export function DiffBody({
  doc,
  path,
  onInject,
  onSessionComment,
  review,
  onExpandGap,
  onRevertLines,
}: DiffBodyProps) {
  const comments = useDiffComments({ anchors: doc.lineMeta, path, review, onSessionComment })
  const { gapped, slots } = comments
  const revert = useBlockRevert(doc, onRevertLines)
  const { revertable, hover, revertBlock } = revert
  const [revertTargets, setRevertTargets] = useState<RevertLine[]>([])
  // Every layer rides the view's identity, so they are memoised together and
  // the editor is rebuilt only when the document behind it actually moved.
  const extra = useMemo(() => {
    const layers = gapped ? [slots.extension] : []
    if (onExpandGap) {
      layers.push(gapExpanders(doc.lineMeta, doc.gaps, onExpandGap))
    }
    if (revertable) {
      const blocks = blockOfLine(doc.lineMeta)
      layers.push(hover.layer({ blocks, buttons: firstLines(blocks), onRevert: revertBlock }))
    }
    return layers.length > 0 ? layers : undefined
  }, [gapped, slots, doc, onExpandGap, revertable, hover, revertBlock])
  const { containerRef, getSelectedDocLines, view } = useDiffEditor(doc, path, extra)
  useSlotSync(view, gapped, slots, comments.wanted)

  return (
    <>
      <InjectMenu
        path={path}
        containerRef={containerRef}
        lineRef={comments.selection?.lines ?? null}
        // Resolve the selection when the menu opens, not on every change.
        onOpenChange={(open) => {
          if (!open) {
            return
          }
          const selected = getSelectedDocLines()
          const range = selected ? newLineRange(doc.lineMeta, selected.from, selected.to) : null
          comments.setSelection(selected && range ? { lines: formatLineRef(range), range } : null)
          setRevertTargets(selected ? revertLinesIn(doc.lineMeta, selected.from, selected.to) : [])
        }}
        onInject={onInject}
        // Both items are offered whenever their destination exists, not only
        // once a range is resolved: the selection is read as the menu opens, so
        // gating them on it would hide them on the very open that produced it.
        // With nothing selected the menu disables them, like Inject lines.
        //
        // Deliberately new-file lines only: a comment on a deleted line is a
        // thread GitHub anchors on the other side, and that needs its own gesture.
        onSessionComment={onSessionComment && comments.openComposer("session")}
        onReviewComment={review && comments.openComposer("review")}
        onRevert={revertable ? () => revert.revertLines(revertTargets) : undefined}
        revertCount={revertTargets.length}
      />
      {comments.portals}
    </>
  )
}
