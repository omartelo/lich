import {cn} from "@/lib/utils"
import {ProviderIcon} from "@/lib/provider-icons"
import type {SessionKind} from "@/lib/sessions"
import type {SessionStatus} from "@/lib/session-events"

// Ring drawn around the provider icon per processing state: a spinning ring
// while Claude produces output, solid emerald once its turn ends, amber when
// it is blocked on the user.
const RING: Record<SessionStatus, string> = {
  busy: "animate-spin border-muted-foreground/25 border-t-muted-foreground",
  done: "border-emerald-500",
  waiting: "border-amber-500",
}

interface SessionStatusIconProps {
  kind: SessionKind
  // Last reported state from the lich hook; null renders the bare icon.
  status: SessionStatus | null
}

// SessionStatusIcon is a session's provider mark wrapped in a status ring. The
// slot is a fixed size so the icon never shifts as the state changes.
export function SessionStatusIcon({kind, status}: SessionStatusIconProps) {
  return (
    <span className="relative flex size-[1.375rem] shrink-0 items-center justify-center text-muted-foreground">
      {status && (
        <span className={cn("absolute inset-0 rounded-full border-[0.09375rem]", RING[status])} />
      )}
      <ProviderIcon kind={kind} size={14} />
    </span>
  )
}
