import { GitFork } from "lucide-react"
import { ContextMenuItem } from "@/components/ui/context-menu"
import { forkUnavailableReason, forkableSession, type Session } from "@/lib/session/sessions"

interface SessionForkItemProps {
  session: Session
  onFork: () => void
}

// "Fork to worktree…" on a session card: live where the provider's own CLI
// branches a conversation, dead under the sentence naming why on the five that
// only resume (providers.SupportsFork). The row stays either way, because a
// user who forks a Claude Code card and finds nothing on their Crush card has
// no way to learn that the offer was withheld rather than missing.
//
// Off the card entirely only where the answer is neither: a shell, and a
// forkable session that has yet to report a conversation to branch.
export function SessionForkItem({ session, onFork }: SessionForkItemProps) {
  const reason = forkUnavailableReason(session)
  if (reason !== null) {
    return (
      <ContextMenuItem disabled>
        <GitFork />
        <span className="flex flex-col items-start">
          Fork to worktree…
          <span className="text-xs">{reason}</span>
        </span>
      </ContextMenuItem>
    )
  }
  if (forkableSession(session) === null) {
    return null
  }
  return (
    <ContextMenuItem onClick={onFork}>
      <GitFork />
      Fork to worktree…
    </ContextMenuItem>
  )
}
