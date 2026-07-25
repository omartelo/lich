import { ProviderIcon } from "./ProviderIcon"
import { formatModel } from "@/lib/model-name"
import { useSessionUsage } from "@/lib/session/use-session-usage"

interface SessionModelProps {
  sessionId: string
}

// SessionModel is the footer's AI-session slot: which model the active session
// runs, read from the same usage report as the context ring (so it appears once
// a Claude session has taken a turn, null before). Self-contained on purpose —
// it reads its own data by id, so the slot can grow with more per-session detail
// without touching FooterBar. Usage is Claude-only today, so the mark is
// Claude's; a second provider that reports usage makes the kind dynamic here.
export function SessionModel({ sessionId }: SessionModelProps) {
  const usage = useSessionUsage(sessionId)
  if (!usage) {
    return null
  }
  return (
    <span className="flex items-center gap-1.5">
      <ProviderIcon kind="claude" size={14} />
      {formatModel(usage.model)}
      {usage.effort ? ` · ${usage.effort}` : ""}
    </span>
  )
}
