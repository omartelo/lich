import { readPref, removePref, writePref } from "@/lib/prefs"
import type { DraftReviewComment } from "@/lib/api-types"

// A review being written: the line comments held back until they are submitted
// together, plus the summary that goes above them. It is the one thing on this
// screen that exists only in the page — GitHub has no record of it until the
// review is filed — and the one thing a lost render would destroy.
//
// That is why it lives here and not in the widget that renders it. The diff
// refetches on focus and while checks run, a file collapses (which destroys its
// editor and every comment box inside it), and the whole page reloads on an app
// update. A draft has to survive all three, so the store is module-level and
// mirrored into localStorage on every write.
//
// Keyed by pull request URL, like the viewed ticks beside it: unique across
// projects, so two pull requests never see each other's drafts.
const KEY_PREFIX = "lich.pulls.review."

export interface PendingReview {
  /** The summary written above the line comments. */
  body: string
  comments: DraftReviewComment[]
}

const EMPTY: PendingReview = { body: "", comments: [] }

const drafts = new Map<string, PendingReview>()
const listeners = new Set<() => void>()

/** The draft review for one pull request; a stable reference between changes. */
export function pendingReview(pullRequest: string): PendingReview {
  const held = drafts.get(pullRequest)
  if (held) {
    return held
  }
  const restored = restore(pullRequest)
  if (restored) {
    drafts.set(pullRequest, restored)
    return restored
  }
  return EMPTY
}

export function subscribePendingReview(listener: () => void): () => void {
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
  }
}

/** Add one line comment to the review being written. */
export function addDraftComment(pullRequest: string, comment: DraftReviewComment): void {
  const current = pendingReview(pullRequest)
  write(pullRequest, { ...current, comments: [...current.comments, comment] })
}

/** Drop the comment at index — the draft's "never mind". */
export function removeDraftComment(pullRequest: string, index: number): void {
  const current = pendingReview(pullRequest)
  if (index < 0 || index >= current.comments.length) {
    return
  }
  write(pullRequest, {
    ...current,
    comments: current.comments.filter((_, at) => at !== index),
  })
}

/** Replace the text of the comment at index, so an edit does not reorder it. */
export function editDraftComment(pullRequest: string, index: number, body: string): void {
  const current = pendingReview(pullRequest)
  const comment = current.comments[index]
  if (!comment || comment.body === body) {
    return
  }
  write(pullRequest, {
    ...current,
    comments: current.comments.map((held, at) => (at === index ? { ...held, body } : held)),
  })
}

export function setReviewBody(pullRequest: string, body: string): void {
  const current = pendingReview(pullRequest)
  if (current.body === body) {
    return
  }
  write(pullRequest, { ...current, body })
}

/** Clear the draft — what a submitted review leaves behind. */
export function clearPendingReview(pullRequest: string): void {
  if (!drafts.has(pullRequest) && restore(pullRequest) === null) {
    return
  }
  drafts.delete(pullRequest)
  removePref(KEY_PREFIX + pullRequest)
  notify()
}

/** The draft comments on one file and side, with the index each one holds in
 * the review — the widget needs the index to edit or drop its own comment. */
export function draftsOnFile(
  review: PendingReview,
  path: string,
  side: string,
): { comment: DraftReviewComment; index: number }[] {
  const out: { comment: DraftReviewComment; index: number }[] = []
  review.comments.forEach((comment, index) => {
    if (comment.path === path && (comment.side || "RIGHT") === (side || "RIGHT")) {
      out.push({ comment, index })
    }
  })
  return out
}

function write(pullRequest: string, next: PendingReview): void {
  drafts.set(pullRequest, next)
  writePref(KEY_PREFIX + pullRequest, JSON.stringify(next))
  notify()
}

function notify(): void {
  for (const listener of listeners) {
    listener()
  }
}

// restore reads a draft written by an earlier page. Anything that does not
// parse as this shape is dropped rather than repaired: a half-written draft
// that renders as a comment box with no line behind it is worse than a draft
// that is simply gone.
function restore(pullRequest: string): PendingReview | null {
  const raw = readPref(KEY_PREFIX + pullRequest)
  if (!raw) {
    return null
  }
  try {
    return parsePendingReview(JSON.parse(raw))
  } catch {
    return null
  }
}

/** Narrow a stored value to a draft, or null. Exported for the suite: this is
 * the boundary where another build's shape, or a hand-edited store, arrives. */
export function parsePendingReview(value: unknown): PendingReview | null {
  if (typeof value !== "object" || value === null) {
    return null
  }
  const held = value as Partial<PendingReview>
  const body = typeof held.body === "string" ? held.body : ""
  const comments = Array.isArray(held.comments)
    ? held.comments.filter(isDraftComment).map(normaliseComment)
    : []
  return body === "" && comments.length === 0 ? null : { body, comments }
}

function isDraftComment(value: unknown): value is DraftReviewComment {
  if (typeof value !== "object" || value === null) {
    return false
  }
  const held = value as Partial<DraftReviewComment>
  return (
    typeof held.path === "string" &&
    held.path !== "" &&
    typeof held.line === "number" &&
    held.line > 0 &&
    typeof held.body === "string" &&
    held.body !== ""
  )
}

// normaliseComment fills what an older draft may not carry, so the rest of the
// app can read the fields without re-checking them.
function normaliseComment(comment: DraftReviewComment): DraftReviewComment {
  return {
    path: comment.path,
    line: comment.line,
    startLine:
      typeof comment.startLine === "number" && comment.startLine > 0 ? comment.startLine : 0,
    side: comment.side === "LEFT" ? "LEFT" : "RIGHT",
    body: comment.body,
  }
}

/** Reset the in-memory half. Tests only: the page never wants this, since a
 * draft outliving a component is the whole point. */
export function resetPendingReviews(): void {
  drafts.clear()
  notify()
}
