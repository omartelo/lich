import type { DraftReviewComment, ReviewThread as Thread } from "@/lib/api-types"
import { COMPOSER_KEY, draftSlotKey, threadSlotKey } from "@/lib/pulls/review-slots"
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

/** The comment being written: the lines it covers and the text so far. Held by
 * the diff body rather than by the box, so a refetch that rebuilds the editor
 * moves the box without emptying it. */
export interface Composer {
  start: number
  end: number
  body: string
}

interface ReviewSlotProps {
  slotKey: string
  review: DiffReview
  composer: Composer | null
  onComposerChange: (body: string) => void
  onComposerSubmit: () => void
  onComposerCancel: () => void
}

// What goes in one gap, resolved from the key the widget carries. A key with
// nothing behind it renders nothing: the slots and the data behind them are
// updated in separate passes, and a thread resolved in between must not throw
// on the way out.
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
    return (
      <div className="my-1.5 flex flex-col gap-1.5 rounded-md bg-sidebar px-3 py-2.5">
        <span className="text-xs text-muted-foreground">
          {composer.start === composer.end
            ? `Comment on line ${composer.end}`
            : `Comment on lines ${composer.start}–${composer.end}`}
        </span>
        <CommentBox
          value={composer.body}
          onChange={onComposerChange}
          onSubmit={onComposerSubmit}
          onCancel={onComposerCancel}
          submitLabel="Add to review"
          placeholder="Leave a comment. It is sent when you submit the review."
          autoFocus
        />
      </div>
    )
  }

  if (slotKey.startsWith("t:")) {
    const thread = review.threads.find((held) => threadSlotKey(held) === slotKey)
    return thread ? (
      <ReviewThread thread={thread} actions={review.actions} className="my-1.5" />
    ) : null
  }

  const drafts = review.drafts.filter(({ comment }) => draftSlotKey(comment) === slotKey)
  return drafts.length > 0 ? (
    <div className="my-1.5">
      <PendingComments drafts={drafts} onEdit={review.onEdit} onRemove={review.onRemove} />
    </div>
  ) : null
}
