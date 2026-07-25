import { RotateCcw } from "lucide-react"
import type { ReactNode } from "react"
import { Button } from "@/components/ui/button"

interface StepperProps {
  value: number
  /** Rendered inside the value box — the number with its unit ("14px", "90%"). */
  display: string
  min: number
  max: number
  step: number
  /** What Reset returns to; the button greys out once value is already there. */
  fallback: number
  onChange: (next: number) => void
  /** Glyphs for decrement and increment — zoom uses magnifiers, size ∓. */
  decrementIcon: ReactNode
  incrementIcon: ReactNode
  decrementLabel: string
  incrementLabel: string
}

// The numeric setting control: two icon buttons flanking a bordered value box,
// with a Reset that disappears into disabled once the value is the default.
// Both Appearance controls (interface zoom, terminal text size) are this.
export function Stepper({
  value,
  display,
  min,
  max,
  step,
  fallback,
  onChange,
  decrementIcon,
  incrementIcon,
  decrementLabel,
  incrementLabel,
}: StepperProps) {
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
      <div className="flex h-9 min-w-16 items-center justify-center rounded-lg border border-border px-3 text-sm tabular-nums text-foreground">
        {display}
      </div>
      <Button
        variant="outline"
        size="icon"
        aria-label={incrementLabel}
        disabled={value >= max}
        onClick={() => onChange(value + step)}
      >
        {incrementIcon}
      </Button>
      <Button variant="ghost" disabled={value === fallback} onClick={() => onChange(fallback)}>
        <RotateCcw />
        Reset
      </Button>
    </div>
  )
}
