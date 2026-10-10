import { useT } from "@/lib/i18n/i18n"
import { cn } from "@/lib/utils"
import { SESSION_PHASES, type SessionPhase } from "@/lib/session/session-filter"

interface SessionPhaseChipsProps {
  picked: ReadonlySet<SessionPhase>
  counts: Record<SessionPhase, number>
  onChange: (picked: ReadonlySet<SessionPhase>) => void
}

// SessionPhaseChips narrows the sidebar by what each card's ring says. Several
// can be on at once, and All is lit while none is, clearing them when clicked.
// Drawn like the command palette's filter tabs, so a chip with nothing in it is
// dimmed rather than dropped: a row that reshuffles as sessions change state
// costs more to aim at than the space it saves.
export function SessionPhaseChips({ picked, counts, onChange }: SessionPhaseChipsProps) {
  const t = useT()
  const toggle = (phase: SessionPhase) => {
    const next = new Set(picked)
    if (!next.delete(phase)) {
      next.add(phase)
    }
    onChange(next)
  }
  return (
    <div
      role="group"
      aria-label={t("sidebar.sessionPhaseChips.group")}
      className="flex flex-wrap items-center gap-0.5"
    >
      <Chip
        label={t("sidebar.sessionPhaseChips.all")}
        // Every session the query matches sits in exactly one phase.
        count={SESSION_PHASES.reduce((sum, phase) => sum + counts[phase], 0)}
        on={picked.size === 0}
        onClick={() => onChange(new Set())}
      />
      {SESSION_PHASES.map((phase) => (
        <Chip
          key={phase}
          label={t(`sidebar.sessionPhaseChips.phase.${phase}`)}
          count={counts[phase]}
          on={picked.has(phase)}
          onClick={() => toggle(phase)}
        />
      ))}
    </div>
  )
}

function Chip({
  label,
  count,
  on,
  onClick,
}: {
  label: string
  count: number
  on: boolean
  onClick: () => void
}) {
  return (
    <button
      type="button"
      aria-pressed={on}
      // The filter field keeps the focus, so typing continues straight after a
      // chip is clicked.
      onMouseDown={(event) => event.preventDefault()}
      onClick={onClick}
      className={cn(
        "inline-flex items-baseline gap-1 rounded-md px-1.5 py-0.5 text-xs outline-none focus-visible:ring-2 focus-visible:ring-ring",
        on ? "bg-accent text-accent-foreground" : "text-muted-foreground hover:bg-accent/50",
        !on && count === 0 && "opacity-40",
      )}
    >
      {label}
      <span className="font-mono text-[0.625rem] tabular-nums opacity-70">{count}</span>
    </button>
  )
}
