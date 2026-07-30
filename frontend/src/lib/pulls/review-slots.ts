import type { ThreadSlot } from "@/lib/codemirror-threads"
import type { DraftReviewComment, ReviewThread } from "@/lib/api-types"
import { docLineAt, type DiffLine, type DiffSide } from "@/lib/git/diff"

// Which gaps one file's diff has to open, and what goes in each. Kept apart from
// the components so the mapping — a GitHub line number to the document line
// rendering it — is testable without a DOM.
//
// A key says what the gap holds, because the widget hands back nothing but the
// element and its key:
//
//   t:<node id>            a thread GitHub already has
//   d:<side>:<start>-<end> the comments written here and not yet sent
//   c                      the box a comment is being typed in
//
// Threads and drafts on the same line get their own gaps; two drafts on the same
// line share one, which is why the draft key is the line rather than an index
// into the review (an index moves when an earlier comment is discarded, and a
// key that moves takes the widget's DOM — and the text inside it — with it).
//
// The composer is not resolved here: it hangs off the *document* line the
// selection ended on, which is what the caller has, and a selection ending on a
// deleted line has no new-file line to resolve at all.
export const COMPOSER_KEY = "c"

const side = (value: string): DiffSide => (value === "LEFT" ? "LEFT" : "RIGHT")

/** The gap the comments on one line share. */
export function draftSlotKey(comment: DraftReviewComment): string {
  const start = comment.startLine > 0 ? comment.startLine : comment.line
  return `d:${side(comment.side)}:${start}-${comment.line}`
}

/** The gap a thread hangs in. */
export function threadSlotKey(thread: ReviewThread): string {
  return `t:${thread.id}`
}

export interface ReviewSlotInput {
  lineMeta: DiffLine[]
  /** Threads anchored in this file. */
  threads: ReviewThread[]
  /** Draft comments in this file. */
  drafts: DraftReviewComment[]
}

// reviewSlots resolves every gap this file's diff should open, in document
// order. Anything the diff cannot anchor is left out rather than guessed at: an
// outdated thread has a line number that addresses code this diff no longer
// shows, and a thread on an unchanged line has no line here at all. Both are
// rendered in the Conversation tab, where nothing pretends to point at code.
export function reviewSlots({ lineMeta, threads, drafts }: ReviewSlotInput): ThreadSlot[] {
  const slots: ThreadSlot[] = []
  const seen = new Set<string>()
  const add = (key: string, line: number, at: DiffSide): void => {
    if (seen.has(key)) {
      return
    }
    const docLine = docLineAt(lineMeta, at, line)
    if (docLine !== null) {
      seen.add(key)
      slots.push({ key, docLine })
    }
  }

  for (const thread of threads) {
    if (!thread.isOutdated) {
      add(threadSlotKey(thread), thread.line, side(thread.side))
    }
  }
  for (const comment of drafts) {
    add(draftSlotKey(comment), comment.line, side(comment.side))
  }
  return slots
}

/** Whether a thread can be rendered against this file's diff at all — the
 * question the Conversation tab asks in reverse, so nothing is shown twice. */
export function isAnchored(thread: ReviewThread, lineMeta: DiffLine[]): boolean {
  return !thread.isOutdated && docLineAt(lineMeta, side(thread.side), thread.line) !== null
}
