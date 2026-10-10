import type { ReactElement, ReactNode } from "react"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"

interface HintProps {
  /** Nothing to say renders the child bare, so a conditional reason needs no branch. */
  label: ReactNode
  side?: "top" | "bottom" | "left" | "right"
  /** One element that forwards its ref: a DOM node, or a ui/ part built with forwardRef. */
  children: ReactElement
}

// The app's one short tooltip, laid over a control that already exists. A
// disabled Button takes no pointer events, so a hint explaining why it is
// disabled goes on a <span> around it.
export function Hint({ label, side, children }: HintProps) {
  if (label === null || label === undefined || label === "") {
    return children
  }
  return (
    <Tooltip>
      <TooltipTrigger render={children} />
      <TooltipContent side={side}>{label}</TooltipContent>
    </Tooltip>
  )
}
