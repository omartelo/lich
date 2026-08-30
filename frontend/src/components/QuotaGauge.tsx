import type { QuotaWindow } from "@/lib/api-types"
import { formatWindow, timeLeft } from "@/lib/quota/quota-format"
import { cn } from "@/lib/utils"
import { usageColor } from "./ContextRing"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"

// Every column is fixed but the bar's. Sized to its own text, the readout gave a
// window with no reset time the width the rows beside it spent on "6d 18h", and
// three gauges of the same thing ended at three different places. The bar is the
// only thing here whose length means something, so it is the only one that
// varies — and the header row shares this grid so the columns line up with it.
export const gaugeGrid = "grid grid-cols-[7rem_minmax(0,1fr)_2.5rem_4rem] items-center gap-x-3"

interface QuotaGaugeProps {
  window: QuotaWindow
  /** Wall clock, so the time to reset re-reads on the minute with everything
   * else on screen rather than on a timer of its own. */
  now: Date
  /** Stack the label over the bar, for the narrow column a tooltip has. The
   * settings screen has the width to put them on one line. */
  stacked?: boolean
}

// QuotaGauge is one metered window: how full it is, and how long until it starts
// over. It takes the colour ramp the context ring and the cost readout use, so a
// window three-quarters spent reads the same everywhere.
//
// The prop is renamed on the way in: `window` is the global every browser
// module already has, and shadowing it inside a component is a trap for the
// next edit that reaches for a timer.
export function QuotaGauge({ window: quota, now, stacked }: QuotaGaugeProps) {
  const length = formatWindow(quota.seconds)
  const left = timeLeft(quota.resetsAt, now)
  const label = length ? `${quota.label} · ${length}` : quota.label
  const bar = (
    <span className="h-1.5 overflow-hidden rounded-full bg-muted">
      <span
        className="block h-full rounded-full bg-current"
        style={{ width: `${quota.percent}%` }}
      />
    </span>
  )
  const reading = quota.lockedReason ? (
    <LockedReading reason={quota.lockedReason} />
  ) : (
    `${quota.percent}%`
  )

  if (stacked) {
    return (
      <div className={cn("flex flex-col gap-1 text-xs", usageColor(quota.percent))}>
        <div className="flex items-baseline justify-between gap-3">
          <span className="text-muted-foreground">{label}</span>
          <span className="tabular-nums">
            {reading}
            {left && ` · ${left}`}
          </span>
        </div>
        {bar}
      </div>
    )
  }
  return (
    <div className={cn(gaugeGrid, "text-xs", usageColor(quota.percent))}>
      <span className="truncate text-muted-foreground">{label}</span>
      {bar}
      <span className="text-right tabular-nums">{reading}</span>
      <span className="text-right tabular-nums text-muted-foreground">{left}</span>
    </div>
  )
}

// LockedReading replaces the bare percentage when the provider says a window
// is locked regardless of how full it reads — the percentage alone would look
// like headroom that spending cannot actually reach. The reason travels to a
// tooltip rather than inline, since the provider's wording is not written for
// the width a gauge row has.
function LockedReading({ reason }: { reason: string }) {
  return (
    <Tooltip>
      <TooltipTrigger render={<span className="cursor-help underline decoration-dotted" />}>
        Locked
      </TooltipTrigger>
      <TooltipContent side="top" className="border border-border bg-card text-foreground">
        {reason}
      </TooltipContent>
    </Tooltip>
  )
}
