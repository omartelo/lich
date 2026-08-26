import { cn } from "@/lib/utils"
import { ProviderIcon } from "@/components/ProviderIcon"
import type { SessionKind } from "@/lib/session/sessions"
import type { SessionStatus } from "@/lib/session/session-events"

// Ring drawn around the provider icon per processing state: a spinning ring
// while Claude produces output, solid emerald once its turn ends, amber when
// it is blocked on the user.
const RING: Record<SessionStatus, string> = {
  busy: "animate-spin border-muted-foreground/25 border-t-muted-foreground",
  done: "border-emerald-500",
  waiting: "border-amber-500",
}

// A finished turn nobody has read yet keeps the ring at full strength; one that
// has been read fades to the same weight the busy ring's track carries. Same
// color, same shape — the only thing the step says is whether this is news,
// which is the question the ring is there to answer when you come back to a
// sidebar of finished agents. It is an opacity step on a semantic color, the
// way every other weight in the app is (frontend/DESIGN.md).
const READ_RING = "border-emerald-500/30"

interface SessionStatusIconProps {
  kind: SessionKind
  // Last reported state from the lich hook; null renders the bare icon.
  status: SessionStatus | null
  // Whether that state is a finished turn still waiting to be read. Only "done"
  // is ever unread (see useSessionUnread).
  unread: boolean
}

// The slot is a fixed size so the icon never shifts as the state changes.
export function SessionStatusIcon({ kind, status, unread }: SessionStatusIconProps) {
  return (
    <span className="relative flex size-[1.375rem] shrink-0 items-center justify-center text-muted-foreground">
      {status && (
        <span
          className={cn(
            "absolute inset-0 rounded-full border-[0.09375rem]",
            status === "done" && !unread ? READ_RING : RING[status],
          )}
        />
      )}
      <ProviderIcon kind={kind} size={14} />
    </span>
  )
}
