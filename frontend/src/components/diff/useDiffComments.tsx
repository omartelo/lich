import { useEffect, useMemo, useState } from "react"
import type { ReactNode } from "react"
import { createPortal } from "react-dom"
import type { EditorView } from "@codemirror/view"
import {
  threadSlots,
  type SlotElements,
  type ThreadSlot,
  type ThreadSlots,
} from "@/lib/codemirror-threads"
import { docLineAt, type DiffLine } from "@/lib/git/diff"
import { COMPOSER_KEY, reviewSlots } from "@/lib/pulls/review-slots"
import { type Composer, type DiffReview, type DiffSelection, ReviewSlot } from "./ReviewSlots"

const NO_SLOTS: SlotElements = new Map()

interface DiffCommentsInput {
  /** Per document line, what threads and the composer anchor against: the
   * diff's own lines, or a split diff's per-row anchors. */
  anchors: DiffLine[]
  path: string
  review?: DiffReview
  onSessionComment?: (path: string, lines: string, text: string) => void
}

export interface DiffComments {
  /** Whether this diff opens gaps at all, which is only where a comment can be written. */
  gapped: boolean
  slots: ThreadSlots
  /** The gaps the view should hold right now. */
  wanted: ThreadSlot[]
  /** The live gap elements, key → element. */
  elements: SlotElements
  /** The resolved new-file lines a comment would be filed against. */
  selection: DiffSelection | null
  setSelection: (selection: DiffSelection | null) => void
  openComposer: (kind: Composer["kind"]) => () => void
  /** Every gap's React content, portalled into its element. */
  portals: ReactNode
}

// useDiffComments is what a diff body needs to hold review threads and comment
// boxes between its lines, whatever layout draws the lines themselves. The slot
// extension it builds rides the view's identity, so it is created once.
export function useDiffComments({
  anchors,
  path,
  review,
  onSessionComment,
}: DiffCommentsInput): DiffComments {
  const [elements, setElements] = useState<SlotElements>(NO_SLOTS)
  const slots = useMemo(() => threadSlots(setElements), [])
  // The gaps exist wherever a comment can be written, which on the dock's
  // working diff is the session's alone.
  const gapped = Boolean(review || onSessionComment)
  const [selection, setSelection] = useState<DiffSelection | null>(null)
  const [composer, setComposer] = useState<Composer | null>(null)

  // Map the stable file line on every render: expanding a gap shifts document
  // lines, while the line the comment is filed against does not move.
  const composerLine = composer ? (docLineAt(anchors, "RIGHT", composer.range.end) ?? 0) : 0
  const wanted = useMemo((): ThreadSlot[] => {
    const built = review
      ? reviewSlots({
          lineMeta: anchors,
          threads: review.threads,
          drafts: review.drafts.map(({ comment }) => comment),
        })
      : []
    return composerLine > 0 ? [...built, { key: COMPOSER_KEY, docLine: composerLine }] : built
  }, [review, anchors, composerLine])

  const openComposer = (kind: Composer["kind"]) => (): void => {
    if (selection) {
      setComposer({ ...selection, kind, body: "" })
    }
  }

  // Where a written comment goes, which is the whole difference between the two
  // menu items: the batch the session's next prompt carries, or the review
  // GitHub is waiting for.
  const fileComment = (): void => {
    const body = composer?.body.trim() ?? ""
    if (!composer || body === "") {
      return
    }
    if (composer.kind === "session") {
      onSessionComment?.(path, composer.lines, body)
    } else {
      review?.onAdd({
        path,
        line: composer.range.end,
        startLine: composer.range.start === composer.range.end ? 0 : composer.range.start,
        side: "RIGHT",
        body,
      })
    }
    setComposer(null)
  }

  const portals =
    gapped &&
    [...elements].map(([key, element]) =>
      createPortal(
        <ReviewSlot
          slotKey={key}
          review={review}
          composer={composer}
          onComposerChange={(body) => setComposer((held) => held && { ...held, body })}
          onComposerSubmit={fileComment}
          onComposerCancel={() => setComposer(null)}
        />,
        element,
        key,
      ),
    )

  return { gapped, slots, wanted, elements, selection, setSelection, openComposer, portals }
}

// useSlotSync hands a view the gaps it should hold, once there is a view.
export function useSlotSync(
  view: EditorView | null,
  gapped: boolean,
  slots: ThreadSlots,
  wanted: ThreadSlot[],
): void {
  useEffect(() => {
    if (view && gapped) {
      slots.update(view, wanted)
    }
  }, [view, gapped, slots, wanted])
}
