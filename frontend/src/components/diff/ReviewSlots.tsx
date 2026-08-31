import type { DraftReviewComment, ReviewThread as Thread } from "@/lib/api-types"
import type { NewLineRange } from "@/lib/git/diff"
import { COMPOSER_KEY, draftSlotKey, isThreadSlot, threadSlotKey } from "@/lib/pulls/review-slots"
import { CommentBox } from "@/components/pulls/CommentBox"
import { PendingComments, ReviewThread, type ThreadActions } from "@/components/pulls/ReviewThread"

// The pull request review laid over one file's diff. Absent for the dock's
// working diff, which has no pull request behind it and therefore no threads,
// no drafts and nothing to submit.
export interface DiffReview {
  /** Threads anchored in this file. */
  threads: Thread[]
  /** Draft comments in this file, each with its index in the pending review —
   * the index is what editing and discarding address. */
  drafts: { comment: DraftReviewComment; index: number }[]
  actions: ThreadActions
  onAdd: (comment: DraftReviewComment) => void
  onEdit: (index: number, body: string) => void
  onRemove: (index: number) => void
}

/** A resolved selection: what to call the lines and which new-file lines they are.
 * A selection that
 * covers only deleted lines has no new-file range and so never becomes one of
 * these — commenting on the other side needs its own gesture. */
export interface DiffSelection {
  lines: string
  range: NewLineRange
  /** Full-file previews have no expandable gaps, so their document line is a
   * stable anchor. Diff composers remap range.end instead. */
  docLine?: number
}

/** The comment being written, and where it goes when it is filed. Held by the
 * diff body rather than by the box, so a refetch that rebuilds the editor moves
 * the box without emptying it. */
export interface Composer extends DiffSelection {
  /** "session" is held for the next prompt; "review" waits for GitHub. */
  kind: "session" | "review"
  body: string
}

interface ReviewSlotProps {
  slotKey: string
  /** Absent on the dock's working diff, which has threads of no kind — only a
   * composer aimed at the session. */
  review?: DiffReview
  composer: Composer | null
  onComposerChange: (body: string) => void
  onComposerSubmit: () => void
  onComposerCancel: () => void
}

// What goes in one gap, resolved from the key the widget carries. A key with
// nothing behind it renders nothing: the slots and the data behind them are
// updated in separate passes, and a thread resolved in between must not throw
// on the way out.
//
// Nothing in here may carry a vertical margin. CodeMirror measures a block
// widget by its element's own box, so a margin inside is height the gutter
// never hears about, and the line numbers below drift by exactly that much.
// The spacing is the slot's padding (`.cm-thread-slot` in codemirror.ts).
export function ReviewSlot({
  slotKey,
  review,
  composer,
  onComposerChange,
  onComposerSubmit,
  onComposerCancel,
}: ReviewSlotProps) {
  if (slotKey === COMPOSER_KEY) {
    if (!composer) {
      return null
    }
    // The label says where the note is going, because the two destinations look
    // identical once the box is open and only one of them reaches GitHub.
    const session = composer.kind === "session"
    return (
      <div className="flex flex-col gap-1.5 rounded-md bg-sidebar px-3 py-2.5">
        <span className="font-mono text-xs text-muted-foreground">
          {session ? "for the session" : "on the pull request"} · {composer.lines}
        </span>
        <CommentBox
          value={composer.body}
          onChange={onComposerChange}
          onSubmit={onComposerSubmit}
          onCancel={onComposerCancel}
          submitLabel={session ? "Add to batch" : "Add to review"}
          placeholder={
            session
              ? "What should change here? It goes to the session with the rest."
              : "Leave a comment. It is sent when you submit the review."
          }
          autoFocus
          submitOnEnter={session}
        />
      </div>
    )
  }

  if (!review) {
    return null
  }

  if (isThreadSlot(slotKey)) {
    const thread = review.threads.find((held) => threadSlotKey(held) === slotKey)
    return thread ? <ReviewThread thread={thread} actions={review.actions} /> : null
  }

  const drafts = review.drafts.filter(({ comment }) => draftSlotKey(comment) === slotKey)
  return drafts.length > 0 ? (
    <div>
      <PendingComments drafts={drafts} onEdit={review.onEdit} onRemove={review.onRemove} />
    </div>
  ) : null
}
