import { cn } from "@/lib/utils"

// usageColor is the semantic text colour for a share of something bounded, on
// one 0–100 scale: muted at rest, amber as it fills, red near the limit. The
// context ring, the footer percent and the tooltip bar read against it, and so
// does the cost readout against its spend ceiling — a budget half spent and a
// context window half full mean the same thing to the eye, so they must not be
// two palettes.
export function usageColor(percent: number): string {
  if (percent >= 95) {
    return "text-red-500"
  }
  if (percent >= 80) {
    return "text-amber-500"
  }
  return "text-muted-foreground"
}

interface ContextRingProps {
  // Share of the context window in use, 0–100.
  percent: number
  className?: string
}

// ContextRing draws a compact donut filled to `percent` — a pure fill glyph, no
// label (the number lives beside it, at footer size where it is legible). The
// radius 15.9155 makes the circumference exactly 100, so the arc's dash length
// is the percent directly. Strokes use currentColor, so the caller sets the
// colour (see usageColor) once for the ring and its adjacent text.
export function ContextRing({ percent, className }: ContextRingProps) {
  return (
    <svg
      viewBox="0 0 36 36"
      className={cn("size-4 shrink-0", className)}
      role="img"
      aria-label={`Context window ${percent}% used`}
    >
      <circle
        cx="18"
        cy="18"
        r="15.9155"
        fill="none"
        stroke="currentColor"
        strokeWidth="4"
        className="opacity-20"
      />
      <circle
        cx="18"
        cy="18"
        r="15.9155"
        fill="none"
        stroke="currentColor"
        strokeWidth="4"
        strokeDasharray={`${percent} 100`}
        strokeLinecap="round"
        transform="rotate(-90 18 18)"
      />
    </svg>
  )
}
