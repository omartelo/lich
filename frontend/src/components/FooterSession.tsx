import { Timer } from "lucide-react"
import { Terminal } from "@/lib/rpc"
import type { SessionKind } from "@/lib/session/sessions"
import type { FooterItem } from "@/lib/footer-layout"
import type { SessionUsage } from "@/lib/session/session-events"
import { useSessionUsage } from "@/lib/session/use-session-usage"
import { useSessionAgent } from "@/lib/session/use-session-agent"
import { usePlanQuotaFor } from "@/lib/quota/use-plan-quota"
import { useSettings } from "@/providers/settings"
import { useCostReadout } from "@/lib/use-cost-readout"
import { budgetShare, COST_MISS_REASON, formatCost } from "@/lib/session/session-cost"
import { formatHandsOn, handsOnDetail, spellHandsOn } from "@/lib/session/hands-on"
import { useRemoteResource } from "@/lib/use-remote-resource"
import { useNow } from "@/lib/use-now"
import { ContextRing, usageColor } from "./ContextRing"
import { SessionModel } from "./SessionModel"
import { PlanQuota } from "./PlanQuota"
import { FooterReadout } from "./FooterReadout"

interface FooterSessionProps {
  sessionId: string
  kind: SessionKind | ""
}

interface FooterSessionItemProps extends FooterSessionProps {
  item: FooterItem
}

// Each selected reading subscribes on its own, so layout changes never add a
// second usage reader to unrelated controls or to the opposite side.
export function FooterSession({ sessionId, kind, item }: FooterSessionItemProps) {
  switch (item) {
    case "model":
      return <SessionModel sessionId={sessionId} kind={kind} />
    case "context":
      return <SessionContext sessionId={sessionId} />
    case "plan":
      return <SessionPlan sessionId={sessionId} kind={kind} />
    case "cost":
      return <SessionCost sessionId={sessionId} />
    case "handsOn":
      return <HandsOnReadout sessionId={sessionId} kind={kind} />
    case "clock":
      return <FooterClock />
    default:
      return null
  }
}

function SessionContext({ sessionId }: { sessionId: string }) {
  const usage = useSessionUsage(sessionId)
  return usage && usage.window > 0 ? <ContextReadout usage={usage} /> : null
}

function SessionPlan({ sessionId, kind }: FooterSessionProps) {
  const provider = useSessionAgent(sessionId) ?? kind
  const plan = usePlanQuotaFor(provider || undefined, sessionId)
  return plan ? <PlanQuota plan={plan} /> : null
}

function SessionCost({ sessionId }: { sessionId: string }) {
  const usage = useSessionUsage(sessionId)
  const showCost = useCostReadout()
  const { costBudget } = useSettings()
  return showCost && usage ? <CostReadout usage={usage} budget={costBudget} /> : null
}

function ContextReadout({ usage }: { usage: SessionUsage }) {
  return (
    <FooterReadout
      label="Context window"
      className={usageColor(usage.percent)}
      detail={
        <span className="font-mono">
          {usage.tokens.toLocaleString()} / {usage.window.toLocaleString()} tokens
        </span>
      }
    >
      <ContextRing percent={usage.percent} /> {usage.percent}%
    </FooterReadout>
  )
}

function CostReadout({ usage, budget }: { usage: SessionUsage; budget: number }) {
  if (usage.costUsd === null && !usage.costMiss) return null
  return (
    <FooterReadout
      label={usage.costUsd === null ? "Cost unavailable" : "Session cost"}
      className={usageColor(budgetShare(usage.costUsd ?? 0, budget))}
      detail={
        usage.costUsd === null && usage.costMiss ? (
          COST_MISS_REASON[usage.costMiss]
        ) : (
          <div className="flex flex-col gap-1">
            {budget > 0 && (
              <span>
                {formatCost(usage.costUsd ?? 0)} of {formatCost(budget)} budget
              </span>
            )}
            <span>
              API cost across this session's conversations. Subscription charges may differ.
            </span>
          </div>
        )
      }
    >
      {usage.costUsd === null ? "$—" : formatCost(usage.costUsd)}
    </FooterReadout>
  )
}

function HandsOnReadout({ sessionId, kind }: FooterSessionProps) {
  const provider = useSessionAgent(sessionId) ?? kind
  const now = useNow()
  const handsOn = useRemoteResource(
    sessionId ? `${sessionId}:${now.getTime()}` : "",
    () => Terminal.HandsOn(sessionId),
    { empty: 0, resetOn: sessionId },
  )
  if (handsOn.error)
    return (
      <FooterReadout label="Hands-on time" detail="Could not read hands-on time.">
        —
      </FooterReadout>
    )
  if (!formatHandsOn(handsOn.data)) return null
  return (
    <FooterReadout
      label="Hands-on time"
      detail={
        <div className="flex flex-col gap-1">
          <span>{spellHandsOn(handsOn.data)}</span>
          <span>{handsOnDetail(provider)}</span>
        </div>
      }
    >
      <Timer className="size-3.5" aria-hidden="true" /> {formatHandsOn(handsOn.data)}
    </FooterReadout>
  )
}

function FooterClock() {
  const now = useNow()
  return (
    <time dateTime={now.toISOString()} className="whitespace-nowrap tabular-nums">
      {now.toDateString()} · {now.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}
    </time>
  )
}
