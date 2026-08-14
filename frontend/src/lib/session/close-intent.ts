import { isLastWorktreeSession, type Session } from "./sessions"

// What closing a session asks for before its row can go.
export type CloseIntent =
  /** Pinned: the card refuses to close at all. */
  | "refuse"
  /** A turn is still running in there — ask before it is killed. */
  | "confirm-running"
  /** The last session holding a worktree — ask what to do with the checkout. */
  | "ask-worktree"
  /** Nothing at stake: close it. */
  | "close"

// closeIntent answers one step at a time: after the user confirms, the caller
// asks again with running false and gets whatever comes next. The turn is asked
// about first — it is the one thing a close destroys that nothing can bring back
// — and answering it leaves the worktree question free to follow, so the two are
// never on screen together.
export function closeIntent(session: Session, sessions: Session[], running: boolean): CloseIntent {
  if (session.pinned) {
    return "refuse"
  }
  if (running) {
    return "confirm-running"
  }
  return isLastWorktreeSession(sessions, session) ? "ask-worktree" : "close"
}
