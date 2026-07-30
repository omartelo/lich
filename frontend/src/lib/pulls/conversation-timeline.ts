import type {
  PullRequestComment,
  PullRequestConversation,
  PullRequestReview,
  ReviewThread,
} from "@/lib/api-types"

// The Conversation tab reads one pull request in the order things were said:
// verdicts, comments on the pull request itself, and the threads hanging off
// its lines, interleaved rather than filed under three headings. A review that
// asked for a change and the thread that carries the detail belong next to each
// other — separating them is what made the browser necessary in the first place.
//
// Resolved threads come out separately: they are settled, and a settled thread
// in the middle of the timeline is noise the eye has to step over every pass.

export type TimelineItem =
  | { kind: "review"; at: number; review: PullRequestReview }
  | { kind: "comment"; at: number; comment: PullRequestComment }
  | { kind: "thread"; at: number; thread: ReviewThread }

export interface Timeline {
  items: TimelineItem[]
  /** Threads that have been settled, newest first, behind their own count. */
  resolved: ReviewThread[]
}

/** When a thread started — the timestamp it is placed by. */
function threadDate(thread: ReviewThread): number {
  const first = thread.comments?.[0]
  return first ? parsed(first.date) : 0
}

// An unparseable date sorts to the front rather than being dropped: something
// said is worth showing even when GitHub's timestamp is not readable.
function parsed(date: string): number {
  const at = Date.parse(date)
  return Number.isNaN(at) ? 0 : at
}

export function conversationTimeline(conversation: PullRequestConversation | null): Timeline {
  if (!conversation) {
    return { items: [], resolved: [] }
  }
  const items: TimelineItem[] = []
  for (const review of conversation.reviews ?? []) {
    items.push({ kind: "review", at: parsed(review.date), review })
  }
  for (const comment of conversation.comments ?? []) {
    items.push({ kind: "comment", at: parsed(comment.date), comment })
  }
  const resolved: ReviewThread[] = []
  for (const thread of conversation.threads ?? []) {
    if (thread.isResolved) {
      resolved.push(thread)
    } else {
      items.push({ kind: "thread", at: threadDate(thread), thread })
    }
  }
  items.sort((a, b) => a.at - b.at)
  resolved.sort((a, b) => threadDate(b) - threadDate(a))
  return { items, resolved }
}

/** How much the Conversation tab has to show — what its counter reads. */
export function conversationCount(timeline: Timeline): number {
  return timeline.items.length + timeline.resolved.length
}
