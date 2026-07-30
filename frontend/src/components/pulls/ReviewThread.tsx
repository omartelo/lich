import { useState } from "react"
import { CheckCheck, CornerDownRight, MessageSquare, Pencil, Trash2 } from "lucide-react"
import { toast } from "sonner"
import { Markdown } from "@/components/Markdown"
import { Button } from "@/components/ui/button"
import { IconAction } from "@/components/common/IconAction"
import type { DraftReviewComment, ReviewThread as Thread } from "@/lib/api-types"
import { updatedAgo } from "@/lib/pulls/pull-request-list"
import { cn, errorText } from "@/lib/utils"
import { CommentBox } from "./CommentBox"

// What a thread card can do to the thread it renders. Both actions are the
// screen's, not this component's: it knows how to ask, never which pull request
// it is asking about.
export interface ThreadActions {
  reply: (commentID: number, body: string) => Promise<void>
  resolve: (threadID: string, resolved: boolean) => Promise<void>
}

interface ReviewThreadProps {
  thread: Thread
  actions: ThreadActions
  /** Show the file and GitHub's own hunk above the comments — for the
   * Conversation tab, where the diff those lines came from is not on screen. */
  standalone?: boolean
  /** Open a settled thread anyway — for a caller that has already asked for the
   * resolved ones by name. */
  defaultOpen?: boolean
  className?: string
}

// One review thread: what was said, in order, with a way to answer it and a way
// to close it. Rendered twice over — inside the diff, in the gap the widget
// opens under the line, and again in the Conversation tab for the threads no
// line can hold (an outdated one, or a file this diff does not show).
export function ReviewThread({
  thread,
  actions,
  standalone,
  defaultOpen,
  className,
}: ReviewThreadProps) {
  const [replying, setReplying] = useState(false)
  const [draft, setDraft] = useState("")
  const [busy, setBusy] = useState(false)
  // A settled thread folds itself away: it is a decision, not an open question,
  // and left open it takes as much of the diff as the argument it ended.
  const [open, setOpen] = useState(defaultOpen ?? !thread.isResolved)
  const comments = thread.comments ?? []
  const last = comments[comments.length - 1]

  if (!open) {
    return (
      <button
        type="button"
        onClick={() => setOpen(true)}
        className={cn(
          "flex items-center gap-2 self-start rounded-md px-2 py-1 text-xs text-muted-foreground transition-colors hover:bg-accent/50 hover:text-foreground",
          className,
        )}
      >
        <CheckCheck className="size-3.5 text-emerald-500" />
        Resolved
        {!standalone && ` · line ${thread.line}`}
        <span className="text-muted-foreground/70">
          {comments.length} {comments.length === 1 ? "comment" : "comments"}
        </span>
      </button>
    )
  }

  const send = async () => {
    if (!last) {
      return
    }
    setBusy(true)
    try {
      await actions.reply(last.id, draft)
      setDraft("")
      setReplying(false)
    } catch (err: unknown) {
      toast.error(`Reply failed: ${errorText(err)}`)
    } finally {
      setBusy(false)
    }
  }

  const setResolved = async (resolved: boolean) => {
    setBusy(true)
    try {
      await actions.resolve(thread.id, resolved)
    } catch (err: unknown) {
      toast.error(`${resolved ? "Resolve" : "Reopen"} failed: ${errorText(err)}`)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div
      className={cn(
        "flex flex-col gap-2.5 rounded-md bg-sidebar px-3 py-2.5 text-sm",
        thread.isResolved && "opacity-70",
        className,
      )}
    >
      <div className="flex items-center gap-2 text-xs text-muted-foreground">
        <MessageSquare className="size-3.5" />
        {standalone ? (
          <span className="truncate font-mono" title={thread.path}>
            {thread.path}
            {thread.line > 0 && `:${thread.line}`}
          </span>
        ) : (
          <span>Line {thread.line}</span>
        )}
        {thread.isOutdated && <span>· outdated</span>}
        <span className="ml-auto flex items-center gap-1">
          {thread.isResolved && (
            <span className="flex items-center gap-1 text-emerald-500">
              <CheckCheck className="size-3.5" />
              Resolved
            </span>
          )}
          <Button
            size="sm"
            variant="ghost"
            disabled={busy}
            onClick={() => void setResolved(!thread.isResolved)}
          >
            {thread.isResolved ? "Reopen" : "Resolve"}
          </Button>
        </span>
      </div>

      {standalone && last?.diffHunk && (
        <pre className="overflow-x-auto rounded-md bg-background px-3 py-2 font-mono text-xs leading-relaxed text-muted-foreground">
          {last.diffHunk}
        </pre>
      )}

      <div className="flex flex-col gap-2.5">
        {comments.map((comment) => (
          <div key={comment.id} className="flex flex-col gap-0.5">
            <div className="flex items-baseline gap-2 text-xs text-muted-foreground">
              <span className="font-semibold text-foreground">{comment.author || "someone"}</span>
              <span>{updatedAgo(comment.date)}</span>
            </div>
            <Markdown>{comment.body}</Markdown>
          </div>
        ))}
      </div>

      {replying ? (
        <CommentBox
          value={draft}
          onChange={setDraft}
          onSubmit={() => void send()}
          onCancel={() => {
            setDraft("")
            setReplying(false)
          }}
          submitLabel="Reply"
          busy={busy}
          placeholder="Reply to this thread"
          autoFocus
        />
      ) : (
        <button
          type="button"
          onClick={() => setReplying(true)}
          disabled={!last}
          className="flex items-center gap-1.5 self-start rounded-md px-1.5 py-1 text-xs text-muted-foreground transition-colors hover:bg-accent/50 hover:text-foreground"
        >
          <CornerDownRight className="size-3.5" />
          Reply
        </button>
      )}
    </div>
  )
}

interface PendingCommentsProps {
  /** The draft comments on one line, with the index each holds in the review. */
  drafts: { comment: DraftReviewComment; index: number }[]
  onEdit: (index: number, body: string) => void
  onRemove: (index: number) => void
}

// The comments written but not sent: what makes this a review rather than a pile
// of remarks. They are marked as pending on purpose — a comment nobody can see
// yet must not look like one that has been posted.
export function PendingComments({ drafts, onEdit, onRemove }: PendingCommentsProps) {
  const [editing, setEditing] = useState<number | null>(null)
  const [draft, setDraft] = useState("")

  return (
    <div className="flex flex-col gap-2 rounded-md bg-sidebar px-3 py-2.5 text-sm">
      {drafts.map(({ comment, index }) =>
        editing === index ? (
          <CommentBox
            key={index}
            value={draft}
            onChange={setDraft}
            onSubmit={() => {
              onEdit(index, draft)
              setEditing(null)
            }}
            onCancel={() => setEditing(null)}
            submitLabel="Save"
            autoFocus
          />
        ) : (
          <div key={index} className="flex flex-col gap-0.5">
            <div className="flex items-center gap-2 text-xs text-muted-foreground">
              <Pencil className="size-3.5" />
              Pending — sent when you submit the review
              <span className="ml-auto flex items-center gap-0.5">
                <IconAction
                  label="Edit this comment"
                  onClick={() => {
                    setDraft(comment.body)
                    setEditing(index)
                  }}
                >
                  <Pencil className="size-3.5" />
                </IconAction>
                <IconAction label="Discard this comment" onClick={() => onRemove(index)}>
                  <Trash2 className="size-3.5" />
                </IconAction>
              </span>
            </div>
            <Markdown>{comment.body}</Markdown>
          </div>
        ),
      )}
    </div>
  )
}
