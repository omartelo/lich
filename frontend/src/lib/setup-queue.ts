// One-shot "run the worktree setup script" marks, keyed by session id: set by
// the create-worktree flow before the session's PTY exists and consumed once
// by TerminalView on the first Start. Lives in the page, like the paste
// queue — a reload between creating the worktree and mounting its terminal
// drops the mark, degrading to running the setup by hand.

const pending = new Set<string>()

export function queueSetup(sessionId: string): void {
  pending.add(sessionId)
}

export function takeSetup(sessionId: string): boolean {
  return pending.delete(sessionId)
}
