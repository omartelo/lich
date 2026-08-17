import type { QuotaPlan } from "@/lib/api-types"
import { usePlanQuotaFor } from "@/lib/quota/use-plan-quota"
import { useNow } from "@/lib/use-now"
import { QuotaGauge, gaugeGrid } from "@/components/QuotaGauge"
import { cn } from "@/lib/utils"
import { SettingBlock } from "./SettingBlock"

// How each provider's login is redone, for the reading that has none. Both are
// the provider's own command: lich reads the credentials those write and never
// refreshes them itself, so a signed-out reading is only fixed there.
const loginCommand: Record<string, string> = {
  claude: "claude",
  codex: "codex login",
}

// PlanUsageSetting shows what is left of one provider's subscription, inside
// that provider's own settings section. Absent for a provider that meters
// nothing — opencode, oh-my-pi and Crush run on the user's own API keys, where
// there is no plan to report — and until the first reading lands.
export function PlanUsageSetting({ providerId }: { providerId: string }) {
  const plan = usePlanQuotaFor(providerId)
  const now = useNow()
  if (!plan) {
    return null
  }
  return (
    <SettingBlock
      title="Plan usage"
      description="How much of each window your subscription has spent, read from your account and refreshed every 5 minutes while lich is open."
    >
      <PlanBody plan={plan} now={now} />
    </SettingBlock>
  )
}

function PlanBody({ plan, now }: { plan: QuotaPlan; now: Date }) {
  if (plan.status === "signed-out") {
    const command = loginCommand[plan.provider]
    return (
      <p className="text-xs text-muted-foreground">
        Signed out.
        {command && (
          <>
            {" Run "}
            <code className="rounded bg-muted px-1 py-0.5 font-mono text-foreground">
              {command}
            </code>
            {" to read plan usage."}
          </>
        )}
      </p>
    )
  }
  if (plan.status === "error") {
    return (
      <p className="text-xs text-muted-foreground">
        Could not read plan usage. The provider's usage endpoint is undocumented and may have
        changed.
      </p>
    )
  }
  return (
    <div className="flex max-w-prose flex-col gap-2">
      {/* The plan doubles as the table's left header, which is what keeps it from
          floating above the rows as a line of its own. */}
      <div
        className={cn(gaugeGrid, "text-[11px] uppercase tracking-wide text-muted-foreground/80")}
      >
        <span className="truncate normal-case tracking-normal text-muted-foreground">
          {plan.plan}
        </span>
        <span />
        <span className="text-right">used</span>
        <span className="text-right">resets</span>
      </div>
      {(plan.windows ?? []).map((window) => (
        <QuotaGauge key={window.label} window={window} now={now} />
      ))}
    </div>
  )
}
