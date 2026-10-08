import { Hourglass } from "lucide-react"
import { limitLine } from "@/lib/session/limit-line"
import type { SessionLimit } from "@/lib/session/session-events"

// The card's rung for a turn a usage limit ended: amber like the block above it,
// because the session is stopped on something it cannot get past by itself.
export function SessionLimitRung({
  limit,
  scheduledAt,
}: {
  limit: SessionLimit
  scheduledAt: number
}) {
  const line = limitLine(limit, scheduledAt, new Date())
  return (
    <span className="flex w-full min-w-0 items-center gap-1 text-xs text-muted-foreground">
      <Hourglass className="size-3 shrink-0 text-tone-wait" />
      <span className="truncate">
        {line.name}
        {line.next && (
          <>
            {` · ${line.next} `}
            <span className="font-medium text-foreground">{line.when}</span>
          </>
        )}
      </span>
    </span>
  )
}
