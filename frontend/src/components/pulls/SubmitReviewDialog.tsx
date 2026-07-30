import { Check, MessageSquare, X } from "lucide-react"
import type { ReviewEvent } from "@/lib/api-types"
import type { PendingReview } from "@/lib/pulls/pending-review-store"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"

const VERDICTS: Record<ReviewEvent, { title: string; action: string; description: string }> = {
  approve: {
    title: "Approve",
    action: "Approve",
    description: "Sign this pull request off. GitHub refuses an approval of your own.",
  },
  comment: {
    title: "Comment",
    action: "Send review",
    description: "Send the comments without a verdict.",
  },
  request_changes: {
    title: "Request changes",
    action: "Request changes",
    description: "Ask for the changes below before this can land.",
  },
}

interface SubmitReviewDialogProps {
  event: ReviewEvent
  review: PendingReview
  submitting: boolean
  onBodyChange: (body: string) => void
  onCancel: () => void
  onSubmit: () => void
}

// The last step of a review: the summary above the line comments, and the
// verdict that sends them. The comments themselves are not editable here — they
// belong to the lines they were written on, and this dialog is about what the
// review as a whole says.
export function SubmitReviewDialog({
  event,
  review,
  submitting,
  onBodyChange,
  onCancel,
  onSubmit,
}: SubmitReviewDialogProps) {
  const verdict = VERDICTS[event]
  const count = review.comments.length
  // GitHub takes a bare approval, but a review that neither approves nor says
  // anything has nothing in it to send.
  const empty = event !== "approve" && count === 0 && review.body.trim() === ""

  return (
    <Dialog open onOpenChange={(next) => !next && onCancel()}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{verdict.title}</DialogTitle>
          <DialogDescription>{verdict.description}</DialogDescription>
        </DialogHeader>
        <div className="flex flex-col gap-3">
          <textarea
            value={review.body}
            onChange={(e) => onBodyChange(e.target.value)}
            rows={6}
            placeholder="Summary — optional"
            autoFocus
            className="min-h-24 w-full resize-y rounded-md border border-input bg-transparent px-2.5 py-1.5 text-sm shadow-xs outline-none transition-[color,box-shadow] placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 dark:bg-input/30"
          />
          <p className="flex items-center gap-1.5 text-xs text-muted-foreground">
            <MessageSquare className="size-3.5" />
            {count === 0
              ? "No line comments — this sends the summary alone."
              : `${count} line ${count === 1 ? "comment" : "comments"} go with it.`}
          </p>
        </div>
        <DialogFooter>
          <Button variant="ghost" onClick={onCancel} disabled={submitting}>
            Cancel
          </Button>
          <Button onClick={onSubmit} disabled={submitting || empty}>
            {event === "approve" ? (
              <Check />
            ) : event === "request_changes" ? (
              <X />
            ) : (
              <MessageSquare />
            )}
            {submitting ? "Sending…" : verdict.action}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
