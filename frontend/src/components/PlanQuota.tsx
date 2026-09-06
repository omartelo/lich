import type { QuotaPlan } from "@/lib/api-types"
import { Gauge } from "lucide-react"
import { hottestWindow, shortWindow } from "@/lib/quota/quota-format"
import { useNow } from "@/lib/use-now"
import { QuotaGauge } from "./QuotaGauge"
import { usageColor } from "./ContextRing"
import { FooterReadout } from "./FooterReadout"

interface PlanQuotaProps {
  plan: QuotaPlan
}

// A session can spend a different login from lich's own. Keep the account
// beside its windows, and omit the name when the provider cannot identify it.
export function PlanQuota({ plan }: PlanQuotaProps) {
  // A locked window must remain visible even when another window is fuller.
  const hottest =
    plan.status === "ok"
      ? (plan.windows?.find((window) => window.lockedReason) ?? hottestWindow(plan))
      : null
  if (!hottest) return null
  return (
    <FooterReadout
      label="Plan usage"
      title={plan.plan ? `${plan.name} · ${plan.plan}` : plan.name}
      tooltipClassName="py-3"
      className={hottest.lockedReason ? "text-destructive" : usageColor(hottest.percent)}
      detail={<PlanDetails plan={plan} />}
    >
      <Gauge className="size-3.5" aria-hidden="true" />
      {shortWindow(hottest.seconds)} {hottest.lockedReason ? "Locked" : `${hottest.percent}%`}
    </FooterReadout>
  )
}

function PlanDetails({ plan }: PlanQuotaProps) {
  const now = useNow()
  return (
    <div className="flex min-w-52 flex-col gap-2.5">
      {(plan.windows ?? []).map((window) => (
        <div key={window.label} className="flex flex-col gap-1">
          <QuotaGauge window={window} now={now} stacked />
          {window.lockedReason && <p className="text-xs text-destructive">{window.lockedReason}</p>}
        </div>
      ))}
      {plan.account && (
        <span className="break-all border-t border-border pt-2 font-mono text-[11px] text-muted-foreground">
          {plan.account}
        </span>
      )}
    </div>
  )
}
