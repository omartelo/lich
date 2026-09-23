import type { ReactNode } from "react"
import { EXIT_MS } from "@/lib/session/use-closing-sessions"
import { cn } from "@/lib/utils"

interface CardTransitionProps {
  // Closed a moment ago and still drawn (useClosingSessions): the card
  // collapses out.
  leaving: boolean
  // Whether a card mounting now grows in. Only one that arrives after its block
  // is on screen does: the ones the block was built with are not news, and a
  // project switch would animate all of them.
  growIn: boolean
  // The card being dragged, whose box rises over the cards the drag passes.
  lifted: boolean
  // Whether any card of the block is being dragged.
  dragging: boolean
  children: ReactNode
}

// CardTransition is the box a sidebar card opens and closes in. The gap between
// cards lives inside it, so that a card collapsing takes its gap with it.
export function CardTransition({
  leaving,
  growIn,
  lifted,
  dragging,
  children,
}: CardTransitionProps) {
  return (
    <div
      // The exit is timed from JS (the card outlives its session by exactly
      // EXIT_MS), so the transition reads its duration from the same constant
      // rather than a class of its own.
      style={leaving ? { transitionDuration: `${EXIT_MS}ms` } : undefined}
      className={cn(
        "grid origin-top transition-[grid-template-rows,opacity,scale] duration-[160ms] ease-out motion-reduce:transition-none",
        // The pose a card leaves to is the one it grows in from. It is written
        // twice because Tailwind finds classes by reading the source: a
        // starting: variant built from a shared string would never be generated.
        leaving
          ? "pointer-events-none grid-rows-[0fr] scale-[0.6] opacity-0 ease-in"
          : "grid-rows-[1fr] scale-100 opacity-100",
        // The scale makes each box a stacking context, so the card's own z-index
        // stops at it: the box is what has to rise over the cards the drag passes.
        lifted && "relative z-10",
        growIn && "starting:grid-rows-[0fr] starting:scale-[0.6] starting:opacity-0",
      )}
    >
      {/* What the collapse hides. Never while a card is being dragged: the drag
          carries it out of this box, and a clipped card is a card that vanishes
          mid-drag. */}
      <div className={cn("min-h-0 pb-1.5", !dragging && "overflow-hidden")}>{children}</div>
    </div>
  )
}
