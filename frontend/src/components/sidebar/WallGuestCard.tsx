import { CloseButton } from "@/components/common/CloseButton"
import { useT } from "@/lib/i18n/i18n"
import type { Session } from "@/lib/session/sessions"
import { useSessionAgent } from "@/lib/session/use-session-agent"
import { useSessionStatus, useSessionUnread } from "@/lib/session/use-session-status"
import { SessionStatusIcon } from "./SessionStatusIcon"

interface WallGuestCardProps {
  session: Session
  /** The project it belongs to, named on the card: it is not this sidebar's. */
  projectName: string
  onSelect: () => void
  onStopShowing: () => void
}

// A wall member from another project, drawn in this project's block of the wall
// so the block lists every pane it puts on screen. Not a SessionCard: every
// action on that card's menu speaks for the sidebar's project, so a guest gets
// the two things that are true from here — go to it, and take it off the wall.
// Its own card, with the whole menu, is in its own project's sidebar.
export function WallGuestCard({
  session,
  projectName,
  onSelect,
  onStopShowing,
}: WallGuestCardProps) {
  const t = useT()
  const status = useSessionStatus(session.id)
  const unread = useSessionUnread(session.id)
  const agent = useSessionAgent(session.id)
  return (
    <button
      type="button"
      onClick={onSelect}
      className="group relative flex w-full flex-col items-start gap-0.5 rounded-md px-2.5 py-2 pr-7 text-left transition-colors hover:bg-accent/60"
    >
      <span className="flex w-full min-w-0 items-center gap-1.5">
        <SessionStatusIcon kind={agent ?? session.kind} status={status} unread={unread} />
        <span className="truncate text-sm font-medium text-foreground">{session.label}</span>
      </span>
      <span className="w-full truncate pl-5.5 text-xs text-muted-foreground">{projectName}</span>
      <CloseButton
        label={t("sidebar.wallGuestCard.stopShowing", { name: session.label })}
        onClick={(event) => {
          event.stopPropagation()
          onStopShowing()
        }}
        className="absolute right-2 top-2.5"
      />
    </button>
  )
}
