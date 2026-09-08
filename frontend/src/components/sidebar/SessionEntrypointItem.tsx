import { Play } from "lucide-react"
import { ContextMenuItem } from "@/components/ui/context-menu"
import type { Session } from "@/lib/session/sessions"

interface SessionEntrypointItemProps {
  session: Session
  onOpen: () => void
}

// "Entrypoint…" on a session card: live on a terminal, dead under the sentence
// naming why on an agent card, the idiom SessionForkItem sets. The row stays
// either way, because a user who gave one terminal an entrypoint and finds
// nothing on their Claude Code card has no way to learn that the offer was
// withheld rather than missing — on a provider card the entrypoint *is* the
// provider, and the store refuses one there anyway.
export function SessionEntrypointItem({ session, onOpen }: SessionEntrypointItemProps) {
  if (session.kind !== "shell") {
    return (
      <ContextMenuItem disabled>
        <Play />
        <span className="flex flex-col items-start">
          Entrypoint…
          <span className="text-xs">Entrypoints run in terminal sessions.</span>
        </span>
      </ContextMenuItem>
    )
  }
  return (
    <ContextMenuItem onClick={onOpen}>
      <Play />
      Entrypoint…
    </ContextMenuItem>
  )
}
