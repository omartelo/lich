import type { ReactNode } from "react"
import { Button } from "@/components/ui/button"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"

interface StepperProps {
  value: number
  /** Rendered inside the value box — the number with its unit ("14px", "90%"). */
  display: string
  min: number
  max: number
  step: number
  /** What clicking the value returns to. */
  fallback: number
  /** The setting's name, for the reset control's label ("Reset the zoom"). */
  name: string
  onChange: (next: number) => void
  /** Glyphs for decrement and increment — zoom uses magnifiers, size ∓. */
  decrementIcon: ReactNode
  incrementIcon: ReactNode
  decrementLabel: string
  incrementLabel: string
}

// The numeric setting control: two icon buttons flanking the value, which is
// itself the reset. A Reset button beside it spent a permanent third control on
// the rarest action, greyed out for as long as the value was the default, which
// is most of the time. Both Appearance controls (interface zoom, terminal text
// size) are this.
export function Stepper({
  value,
  display,
  min,
  max,
  step,
  fallback,
  name,
  onChange,
  decrementIcon,
  incrementIcon,
  decrementLabel,
  incrementLabel,
}: StepperProps) {
  const custom = value !== fallback
  return (
    <div className="flex items-center gap-2">
      <Button
        variant="outline"
        size="icon"
        aria-label={decrementLabel}
        disabled={value <= min}
        onClick={() => onChange(value - step)}
      >
        {decrementIcon}
      </Button>
      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              variant="outline"
              // Not an icon button: the box is as wide as its longest reading,
              // so stepping never shifts the controls around it. Never
              // disabled either — the reading is what this box is for, and a
              // disabled button greys the number out for as long as the value
              // is the default, which is most of the time. At the default the
              // click is simply a no-op.
              className="min-w-16 tabular-nums"
              aria-label={custom ? `Reset ${name}` : display}
              onClick={() => custom && onChange(fallback)}
            />
          }
        >
          {display}
        </TooltipTrigger>
        <TooltipContent>{custom ? `Reset ${name}` : "Default"}</TooltipContent>
      </Tooltip>
      <Button
        variant="outline"
        size="icon"
        aria-label={incrementLabel}
        disabled={value >= max}
        onClick={() => onChange(value + step)}
      >
        {incrementIcon}
      </Button>
    </div>
  )
}
