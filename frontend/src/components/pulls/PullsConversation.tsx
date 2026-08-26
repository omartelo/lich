import { useState } from "react"
import { Check, CheckCheck, ChevronDown, ChevronRight, MessageSquare, X } from "lucide-react"
import { toast } from "sonner"
import { Markdown } from "@/components/Markdown"
import { Notice } from "@/components/common/Notice"
import type { PullRequestConversation, PullRequestReview } from "@/lib/api-types"
import { conversationTimeline } from "@/lib/pulls/conversation-timeline"
import { useDraft } from "@/lib/pulls/use-draft"
import { errorText } from "@/lib/utils"
import { Byline } from "./Byline"
import { CommentBox } from "./CommentBox"
import { ReviewThread, type ThreadActions } from "./ReviewThread"

interface PullsConversationProps {
  /** The pull request this is about, by URL — what its unsent comment is filed
   * under, so the box survives the tab strip above it (draft-store). */
  pullRequest: string
  conversation: PullRequestConversation | null
  loading: boolean
  actions: ThreadActions
  /** Leave a comment on the pull request itself. */
  onComment: (body: string) => Promise<void>
}

// PullsConversation is the "Conversation" tab: everything said about the pull
// request, oldest first. The threads are here too, in full — the diff renders
// them where their lines are, but a thread whose lines this diff no longer shows
// would otherwise be invisible, and the timeline is where a review is read as a
// conversation rather than as annotations.
export function PullsConversation({
  conversation,
  loading,
  pullRequest,
  actions,
  onComment,
}: PullsConversationProps) {
  const [draft, setDraft] = useDraft("comment", pullRequest)
  const [sending, setSending] = useState(false)
  const [showResolved, setShowResolved] = useState(false)
  const timeline = conversationTimeline(conversation)

  const send = async () => {
    setSending(true)
    try {
      await onComment(draft ?? "")
      setDraft(null)
    } catch (err: unknown) {
      toast.error(`Comment failed: ${errorText(err)}`)
    } finally {
      setSending(false)
    }
  }

  if (loading && !conversation) {
    return <Notice className="px-6 py-5 text-sm">Reading the conversation…</Notice>
  }

  return (
    <div className="flex max-w-3xl flex-col gap-4 px-6 py-5">
      {timeline.items.length === 0 && timeline.resolved.length === 0 && (
        <p className="text-sm text-muted-foreground">
          Nothing said yet. A comment here goes on the pull request itself; a comment on a line goes
          in the diff.
        </p>
      )}

      {timeline.items.map((item) => {
        if (item.kind === "review") {
          return <ReviewVerdict key={`r${item.at}${item.review.author}`} review={item.review} />
        }
        if (item.kind === "comment") {
          return (
            <div key={`c${item.at}${item.comment.author}`} className="flex flex-col gap-1">
              <Byline author={item.comment.author} date={item.comment.date} />
              <Markdown>{item.comment.body}</Markdown>
            </div>
          )
        }
        return (
          <ReviewThread key={item.thread.id} thread={item.thread} actions={actions} standalone />
        )
      })}

      {timeline.resolved.length > 0 && (
        <div className="flex flex-col gap-2">
          <button
            type="button"
            onClick={() => setShowResolved(!showResolved)}
            aria-expanded={showResolved}
            className="flex items-center gap-2 self-start rounded-md px-1.5 py-1 text-xs text-muted-foreground transition-colors hover:bg-accent/50 hover:text-foreground"
          >
            {showResolved ? (
              <ChevronDown className="size-3.5" />
            ) : (
              <ChevronRight className="size-3.5" />
            )}
            <CheckCheck className="size-3.5" />
            {timeline.resolved.length} resolved{" "}
            {timeline.resolved.length === 1 ? "thread" : "threads"}
          </button>
          {showResolved &&
            timeline.resolved.map((thread) => (
              <ReviewThread
                key={thread.id}
                thread={thread}
                actions={actions}
                standalone
                defaultOpen
              />
            ))}
        </div>
      )}

      <div className="flex flex-col gap-1.5 border-t border-border pt-4">
        <span className="text-xs text-muted-foreground">Comment on the pull request</span>
        <CommentBox
          value={draft ?? ""}
          onChange={setDraft}
          onSubmit={() => void send()}
          submitLabel="Comment"
          busy={sending}
          placeholder="Something about the pull request as a whole"
        />
      </div>
    </div>
  )
}

// A verdict is the one thing in the timeline with a colour: it is the answer to
// "can this land", which is what the eye comes here for.
function ReviewVerdict({ review }: { review: PullRequestReview }) {
  const verdict =
    review.state === "APPROVED" ? (
      <span className="flex items-center gap-1 text-emerald-500">
        <Check className="size-3.5" />
        approved
      </span>
    ) : review.state === "CHANGES_REQUESTED" ? (
      <span className="flex items-center gap-1 text-amber-500">
        <X className="size-3.5" />
        requested changes
      </span>
    ) : review.state === "DISMISSED" ? (
      <span className="text-muted-foreground">review dismissed</span>
    ) : (
      <span className="flex items-center gap-1 text-muted-foreground">
        <MessageSquare className="size-3.5" />
        commented
      </span>
    )

  return (
    <div className="flex flex-col gap-1">
      <Byline author={review.author} date={review.date}>
        {verdict}
      </Byline>
      {review.body.trim() !== "" && <Markdown>{review.body}</Markdown>}
    </div>
  )
}
