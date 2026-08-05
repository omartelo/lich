// The line comments written while reading a review, held until they go to the
// session as one prompt. A comment is a place plus what should change there:
// the file, the lines the selection covered, and the note.
//
// Keyed by what is under review — the checkout's path for the working diff, the
// pull request's URL for a PR — so the dock and the Pulls screen never mix their
// notes, and leaving a tab does not drop them.
//
// In memory only, like the Viewed ticks next door: a comment anchors to line
// numbers of the diff as it was rendered, and nothing tracks the file moving
// underneath. Outliving a restart would mean outliving every edit made in
// between, pointing at lines that have since moved.
//
// Module-level store + a subscribe/getSnapshot pair for useSyncExternalStore,
// matching the app's no-state-library pattern. Each snapshot is an array that is
// replaced, never mutated, so subscribers keep a stable reference until
// something actually changes.

import { bracketedPaste } from "./terminal/bracketed-paste"

export interface ReviewComment {
  /** Identity for the list and for removal; carries no meaning of its own. */
  id: string
  path: string
  /** The selection's new-file lines, git-style: "19" or "12-30". */
  lines: string
  text: string
}

const byTarget = new Map<string, readonly ReviewComment[]>()
const listeners = new Set<() => void>()

const empty: readonly ReviewComment[] = []

let nextId = 1

function notify(): void {
  for (const listener of listeners) {
    listener()
  }
}

/** One review's pending comments, in the order they were written. */
export function reviewComments(target: string): readonly ReviewComment[] {
  return byTarget.get(target) ?? empty
}

/** Append a comment. Blank text is not a comment, and is dropped. */
export function addReviewComment(target: string, path: string, lines: string, text: string): void {
  const trimmed = text.trim()
  if (trimmed === "") {
    return
  }
  const comment: ReviewComment = { id: String(nextId++), path, lines, text: trimmed }
  byTarget.set(target, [...reviewComments(target), comment])
  notify()
}

export function removeReviewComment(target: string, id: string): void {
  const current = reviewComments(target)
  const next = current.filter((comment) => comment.id !== id)
  if (next.length === current.length) {
    return
  }
  byTarget.set(target, next)
  notify()
}

export function clearReviewComments(target: string): void {
  if (!byTarget.delete(target)) {
    return
  }
  notify()
}

export function subscribeReviewComments(listener: () => void): () => void {
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
  }
}

/**
 * The batch as one terminal write: the comments as a markdown list, each
 * anchored the way an inject writes a line reference, wrapped as a paste. The
 * block is multi-line, and a newline written straight into a TUI's prompt is a
 * send — one per line, so the agent would answer the first comment while the
 * rest were still being typed.
 */
export function composeReviewComments(list: readonly ReviewComment[]): string {
  const items = list.map((c) => `- ${c.path}:${c.lines} — ${continuation(c.text)}`)
  return bracketedPaste(`Review comments:\n\n${items.join("\n")}`)
}

// A comment written across several lines stays one list item: its own newlines
// are indented under the bullet instead of starting a new one.
function continuation(text: string): string {
  return text.split("\n").join("\n  ")
}
