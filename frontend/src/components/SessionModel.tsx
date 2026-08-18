import { ProviderIcon } from "./ProviderIcon"
import { formatModel } from "@/lib/model-name"
import { useSessionAgent } from "@/lib/session/use-session-agent"
import { useSessionUsage } from "@/lib/session/use-session-usage"
import type { SessionKind } from "@/lib/session/sessions"

interface SessionModelProps {
  sessionId: string
  kind: SessionKind | ""
}

// SessionModel is the footer's AI-session slot: which model the active session
// runs, read from the same usage report as the context ring (so it appears once
// a supported session has taken a turn, null before). Self-contained on purpose —
// it reads its own data by id, so the slot can grow with more per-session detail
// without touching FooterBar. A provider started inside a shell wins over the
// card's persisted kind, matching the session card itself.
export function SessionModel({ sessionId, kind }: SessionModelProps) {
  const usage = useSessionUsage(sessionId)
  const provider = useSessionAgent(sessionId) ?? kind
  if (!usage || !provider) {
    return null
  }
  return (
    <span className="flex items-center gap-1.5">
      <ProviderIcon kind={provider} size={14} />
      {formatModel(usage.model)}
      {usage.effort ? ` · ${usage.effort}` : ""}
    </span>
  )
}
