import { Send, X } from "lucide-react"
import { useSyncExternalStore } from "react"
import { toast } from "sonner"
import { IconAction } from "@/components/common/IconAction"
import { Button } from "@/components/ui/button"
import { splitPath } from "@/lib/git/lang-badge"
import {
  clearReviewComments,
  composeReviewComments,
  removeReviewComment,
  reviewComments,
  subscribeReviewComments,
} from "@/lib/review-comments"

interface CommentBatchProps {
  /** What is under review: the checkout's path, or the pull request's URL. */
  target: string
  /** Write into the session's terminal; false when no session took it. */
  onInject: (text: string) => boolean
}

// CommentBatch is the strip at the foot of a review panel: the comments written
// so far, and the one action that hands them all to the session as a single
// prompt. Nothing at all until the first comment is written.
export function CommentBatch({ target, onInject }: CommentBatchProps) {
  const comments = useSyncExternalStore(subscribeReviewComments, () => reviewComments(target))

  if (comments.length === 0) {
    return null
  }

  // Comments are the one thing here the user typed, so they survive a failed
  // send: only a write the session took clears them.
  const send = () => {
    if (!onInject(composeReviewComments(comments))) {
      toast.error("Open a session to send these comments to")
      return
    }
    clearReviewComments(target)
  }

  return (
    <div className="flex-none border-t border-border">
      <ul className="max-h-40 overflow-y-auto p-1">
        {comments.map((comment) => (
          <li
            key={comment.id}
            className="flex items-center gap-2 rounded-md px-1.5 py-1 text-xs hover:bg-accent/50"
          >
            <span className="shrink-0 font-mono text-muted-foreground" title={comment.path}>
              {splitPath(comment.path).base}:{comment.lines}
            </span>
            <span className="truncate" title={comment.text}>
              {comment.text}
            </span>
            <span className="ml-auto">
              <IconAction
                label="Remove comment"
                onClick={() => removeReviewComment(target, comment.id)}
              >
                <X className="size-3.5" />
              </IconAction>
            </span>
          </li>
        ))}
      </ul>
      <div className="flex items-center gap-2 px-2 pt-1 pb-2">
        <span className="text-xs text-muted-foreground tabular-nums">
          {comments.length} {comments.length === 1 ? "comment" : "comments"}
        </span>
        <span className="ml-auto flex items-center gap-1">
          <Button variant="ghost" size="sm" onClick={() => clearReviewComments(target)}>
            Clear
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={send}
            className="bg-accent/55 text-foreground hover:bg-accent"
          >
            <Send />
            Send
          </Button>
        </span>
      </div>
    </div>
  )
}
