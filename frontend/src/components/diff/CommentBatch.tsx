import { Send, X } from "lucide-react"
import { useState, useSyncExternalStore } from "react"
import { toast } from "sonner"
import { IconAction } from "@/components/common/IconAction"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { splitPath } from "@/lib/git/lang-badge"
import { useT } from "@/lib/i18n/i18n"
import {
  addReviewNote,
  clearReviewComments,
  composeReviewComments,
  removeReviewComment,
  reviewComments,
  subscribeReviewComments,
} from "@/lib/review-comments"

interface CommentBatchProps {
  /** Where the batch is going: the checkout's path, shared by every surface
   * reviewing it. */
  target: string
  /** Write into the session's terminal; false when no session took it. */
  onInject: (text: string) => boolean
}

// CommentBatch is the strip at the foot of a review panel: the comments written
// so far, and the one action that hands them all to the session as a single
// prompt. Nothing at all until the first comment is written.
export function CommentBatch({ target, onInject }: CommentBatchProps) {
  const t = useT()
  const comments = useSyncExternalStore(subscribeReviewComments, () => reviewComments(target))
  const [note, setNote] = useState("")

  if (comments.length === 0) {
    return null
  }

  // A note about the change as a whole, for the remark that belongs with the
  // comments rather than after them. It rides the strip, so it is offered only
  // once there is a batch to join: alone it would be a prompt written the long
  // way round.
  const addNote = () => {
    addReviewNote(target, note)
    setNote("")
  }

  const clear = () => {
    clearReviewComments(target)
    setNote("")
  }

  // Comments are the one thing here the user typed, so they survive a failed
  // send: only a write the session took clears them.
  const send = () => {
    if (!onInject(composeReviewComments(comments, note))) {
      toast.error(t("diff.commentBatch.openSession"))
      return
    }
    clear()
  }

  return (
    <div className="flex-none border-t border-border">
      <ul className="max-h-40 overflow-y-auto p-1">
        {comments.map((comment) => (
          <li
            key={comment.id}
            className="flex items-center gap-2 rounded-md px-1.5 py-1 text-xs hover:bg-accent/50"
          >
            {comment.path !== "" && (
              <span className="shrink-0 font-mono text-muted-foreground" title={comment.path}>
                {splitPath(comment.path).base}:{comment.lines}
              </span>
            )}
            <span className="truncate" title={comment.text}>
              {comment.text}
            </span>
            <span className="ml-auto">
              <IconAction
                label={t("diff.commentBatch.removeComment")}
                onClick={() => removeReviewComment(target, comment.id)}
              >
                <X className="size-3.5" />
              </IconAction>
            </span>
          </li>
        ))}
      </ul>
      <div className="px-2 pt-1">
        <Input
          value={note}
          onChange={(event) => setNote(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === "Enter") {
              event.preventDefault()
              addNote()
            }
          }}
          placeholder={t("diff.commentBatch.notePlaceholder")}
          aria-label={t("diff.commentBatch.noteLabel")}
          className="h-7 text-xs"
        />
      </div>
      <div className="flex items-center gap-2 px-2 pt-1 pb-2">
        <span className="text-xs text-muted-foreground tabular-nums">
          {t("diff.commentBatch.count", { count: comments.length })}
        </span>
        <span className="ml-auto flex items-center gap-1">
          <Button variant="ghost" size="sm" onClick={clear}>
            {t("diff.commentBatch.clear")}
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={send}
            className="bg-accent/55 text-foreground hover:bg-accent"
          >
            <Send />
            {t("diff.commentBatch.send")}
          </Button>
        </span>
      </div>
    </div>
  )
}
