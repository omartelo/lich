// One-shot "branch this conversation" marks, keyed by session id: the value is
// the *parent's* provider conversation id, set by the fork flow before the new
// session's PTY exists and consumed once by TerminalView on the first Start.
// After that spawn the card has a conversation of its own — the provider
// assigns it, the session-start hook reports it — so nothing here is persisted.
//
// Lives in the page, like the setup queue beside it: a reload between creating
// the worktree and mounting its terminal drops the mark, and the session opens
// on an empty conversation instead of a copied one.

const pending = new Map<string, string>()

export function queueFork(sessionId: string, parentConversationId: string): void {
  pending.set(sessionId, parentConversationId)
}

/** The parent conversation to branch on this spawn, or "" for an ordinary one. */
export function takeFork(sessionId: string): string {
  const parent = pending.get(sessionId) ?? ""
  pending.delete(sessionId)
  return parent
}
