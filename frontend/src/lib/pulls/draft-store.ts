import { createKeyedStore } from "@/lib/keyed-store"

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
// In memory only. It survives every unmount this screen can produce, which is
// what was being lost; it does not survive a reload. Persisting is the same
// argument pending-review-store already won — GitHub has no record of an unsent
// comment either — and the reason to hold off is that nothing here would ever
// collect an abandoned reply, where a filed review clears itself on submit. Add
// it when a draft lost to a reload survives to be reported, not before.
export const draftStore = createKeyedStore<string | null>(null)

/** What a draft belongs to. The kind rides the key so two boxes about the same
 * pull request — its description and its conversation — never share one. */
export type DraftKind = "body" | "comment" | "reply"

/** `id` identifies what is being written about: the pull request's URL for a
 * description or a comment (unique across projects, like the viewed ticks and
 * the pending review beside them), and the thread's own id for a reply, which
 * GitHub already makes unique on its own. */
export function draftKey(kind: DraftKind, id: string): string {
  // A newline appears in neither a URL nor a GitHub node id, so no id can be
  // written that lands on another kind's key.
  return `${kind}\n${id}`
}
