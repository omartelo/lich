import { createKeyedStore, type ReadableKeyedStore } from "@/lib/keyed-store"
import { prefKeys, readPref, removePref, writePref } from "@/lib/prefs"

// Prose typed into the pull request screen and not yet sent: a description
// being rewritten, a comment being composed, a reply to a review thread.
//
// CommentBox already refuses to own its text, and says why — the CodeMirror
// editor under a thread widget is rebuilt whenever the diff refetches, so a box
// holding its own state loses it there. That pushed ownership up to the caller,
// which was not far enough: the callers held it in `useState`, and the tab strip
// above them is a ternary between component types (PullRequestView), so every
// trip to Files changed and back destroyed the caller too. Ownership has to
// leave the tree, which is this file.
//
// Keyed rather than one bag, because a screen holds several of these open at
// once — the description, the conversation box, a reply in every thread on the
// diff — and one keystroke must re-render one of them.
//
// null is "no draft", and it is not the same as "": a description being cleared
// is a legitimate edit, and a reply box that is open but empty is still open.
// That distinction was already PullsOverview's, and it now carries the reply
// box's `replying` flag too, which is one state fewer to keep in step.
//
// Mirrored into localStorage on every write, like the pending review beside it
// (pending-review-store): GitHub has no record of an unsent comment either, and
// an app update reloading the page is not an answer to "I was writing that".
// What the review does not need is collection — a filed review clears itself on
// submit, while a reply abandoned on a thread nobody returns to has nothing to
// retire it. That is `sweepDrafts` below, and it is why the storage key carries
// the project and the pull request number rather than a URL: the list column
// already reads a project's open pull requests, and a number that is no longer
// among them is a draft with nowhere to go.
const KEY_PREFIX = "lich.pulls.draft."

// The backstop under that sweep. A project the user closes, or a repository
// nobody opens again, lands no list to sweep against, so its drafts would sit
// in storage forever. Long enough that a review picked up after a holiday is
// still there, short enough that "forever" is not the answer.
export const DRAFT_MAX_AGE_MS = 30 * 24 * 60 * 60 * 1000

const store = createKeyedStore<string | null>(null)

/** The drafts, read side. Writing goes through `setDraft`, which is what keeps
 * storage in step with memory. */
export const draftStore: ReadableKeyedStore<string | null> = store

/** What a draft belongs to: the project whose list retires it, and the pull
 * request it is about. */
export interface DraftScope {
  projectId: string
  number: number
}

/** What a draft is on. The kind rides the key so two boxes about the same pull
 * request — its description and its conversation — never share one. */
export type DraftKind = "body" | "comment" | "reply"

/** The key one box's prose is filed under, in the store and in storage alike.
 * `id` names the thread a reply answers; the description and the comment are
 * about the pull request itself and take none. */
export function draftKey(scope: DraftScope, kind: DraftKind, id = ""): string {
  const target = id ? `${kind}.${id}` : kind
  return `${KEY_PREFIX}${scope.projectId}.${scope.number}.${target}`
}

/** Write one box's prose. null (sent, or the box closed) and "" (emptied) are
 * both drafts with nothing to keep, so both collect the stored copy — the
 * distinction between them is the screen's, and stays in memory. */
export function setDraft(key: string, text: string | null): void {
  store.set(key, text)
  if (text) {
    writePref(key, JSON.stringify({ text, at: Date.now() }))
  } else {
    removePref(key)
  }
}

/** Retire the stored drafts this project has no pull request for any more, and
 * anything anywhere past the age cap. `open` is the project's open pull
 * requests, as the list column read them.
 *
 * Storage only: a box still on screen keeps what was typed into it, because a
 * pull request merged while its reply was being written is not a reason to
 * empty the box under the user. */
export function sweepDrafts(projectId: string, open: readonly number[]): void {
  const keep = new Set(open)
  // A list says nothing about a number above its own highest. Numbers are
  // monotonic, so a pull request opened after the list was read — and the
  // screen paints from an answer that can be minutes old (remote-cache) — sits
  // above everything in it, and would otherwise read as one that had closed.
  // A list that came back empty carries no such ceiling and so retires
  // nothing; that repository's drafts are the age cap's.
  const ceiling = Math.max(0, ...open)
  const now = Date.now()
  for (const key of prefKeys(KEY_PREFIX)) {
    const scope = scopeOf(key)
    const closed =
      scope !== null &&
      scope.projectId === projectId &&
      scope.number <= ceiling &&
      !keep.has(scope.number)
    if (closed || parseDraft(readPref(key), now) === null) {
      removePref(key)
    }
  }
}

// The project and pull request a stored key is about, or null for a key this
// build cannot read as one — a key from another build, or a hand-edited one,
// belongs to no project and so is swept by the age cap alone.
function scopeOf(key: string): DraftScope | null {
  const parts = key.slice(KEY_PREFIX.length).split(".")
  const number = Number(parts[1])
  if (parts.length < 3 || !parts[0] || !Number.isInteger(number) || number <= 0) {
    return null
  }
  return { projectId: parts[0], number }
}

/** Narrow a stored value to the prose in it, or null — which is also what an
 * expired draft reads as. Anything that does not parse is dropped rather than
 * repaired, like the pending review beside it: half a comment restored into a
 * box is worse than a box that is simply empty. */
export function parseDraft(raw: string | null, now: number): string | null {
  if (!raw) {
    return null
  }
  try {
    const held = JSON.parse(raw) as { text?: unknown; at?: unknown }
    const text = typeof held.text === "string" ? held.text : ""
    // A record with no timestamp cannot be aged, so it is treated as expired
    // rather than kept forever — the one thing this file must not allow.
    const at = typeof held.at === "number" ? held.at : 0
    return text !== "" && now - at < DRAFT_MAX_AGE_MS ? text : null
  } catch {
    return null
  }
}

// What an earlier page left behind, read once at load: the reload these drafts
// used not to survive. Expired and unreadable records are collected on the way
// past, so a store nobody sweeps still shrinks.
function restoreDrafts(): void {
  const now = Date.now()
  for (const key of prefKeys(KEY_PREFIX)) {
    const text = parseDraft(readPref(key), now)
    if (text === null) {
      removePref(key)
    } else {
      store.set(key, text)
    }
  }
}

restoreDrafts()
