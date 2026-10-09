import { Play } from "lucide-react"
import { ContextMenuItem } from "@/components/ui/context-menu"
import { useT } from "@/lib/i18n/i18n"
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
  const t = useT()
  if (session.kind !== "shell") {
    return (
      <ContextMenuItem disabled>
        <Play />
        <span className="flex flex-col items-start">
          {t("sidebar.sessionEntrypointItem.label")}
          <span className="text-xs">{t("sidebar.sessionEntrypointItem.shellOnly")}</span>
        </span>
      </ContextMenuItem>
    )
  }
  return (
    <ContextMenuItem onClick={onOpen}>
      <Play />
      {t("sidebar.sessionEntrypointItem.label")}
    </ContextMenuItem>
  )
}
