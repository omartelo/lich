import { Gauge } from "lucide-react"
import { hottestWindow, shortWindow } from "@/lib/quota/quota-format"
import { usePlanQuotaFor } from "@/lib/quota/use-plan-quota"
import type { SessionKind } from "@/lib/session/sessions"
import { useNow } from "@/lib/use-now"
import { cn } from "@/lib/utils"
import { usageColor } from "./ContextRing"
import { QuotaGauge } from "./QuotaGauge"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"

interface PlanQuotaProps {
  /** Which provider the active session runs; "" when none is active. */
  kind: SessionKind | ""
  /** Whose plan to read: the active session, which may spend an account of its
   * own when it runs a binary the user configured. */
  sessionId: string
}

// PlanQuota is the footer's plan-usage slot: how much of the active session's
// subscription is spent, with every window behind a tooltip. Self-contained like
// SessionModel — it resolves its own reading from the provider id.
//
// One window is shown, and it is the fullest one: a weekly cap about to run out
// must not hide behind a session window that reset an hour ago. Nothing renders
// at all for a provider that meters no subscription, while the reading is
// signed out or failed — the Settings screen is where a login is fixed, and a
// status strip is the wrong place to be told to run a command — or for a
// session whose account lich could not identify, where any number would be
// somebody else's.
export function PlanQuota({ kind, sessionId }: PlanQuotaProps) {
  const plan = usePlanQuotaFor(kind || undefined, sessionId)
  const now = useNow()
  const hottest = plan && plan.status === "ok" ? hottestWindow(plan) : null
  if (!plan || !hottest) {
    return null
  }
  const length = shortWindow(hottest.seconds)
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <span
            className={cn("flex items-center gap-1.5 tabular-nums", usageColor(hottest.percent))}
          />
        }
      >
        <Gauge className="size-3.5 shrink-0" aria-hidden="true" />
        {length && <span className="opacity-70">{length}</span>}
        {hottest.percent}%
      </TooltipTrigger>
      <TooltipContent side="top" className="border border-border bg-card text-foreground">
        <div className="flex min-w-52 flex-col gap-2.5">
          <span className="font-medium">
            {plan.plan ? `${plan.name} · ${plan.plan}` : plan.name}
          </span>
          {(plan.windows ?? []).map((w) => (
            <QuotaGauge key={w.label} window={w} now={now} stacked />
          ))}
        </div>
      </TooltipContent>
    </Tooltip>
  )
}
