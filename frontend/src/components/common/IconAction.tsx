import type { ReactNode } from "react"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"

interface IconAction {
  /** Both the tooltip and the accessible name — one string, one meaning. */
  label: string
  onClick: () => void
  children: ReactNode
}

// A bare icon button for a panel's header strip: no border, no fill at rest,
// its meaning carried by a tooltip. Attach as context, discard changes, collapse
// all, full screen, hide the tree.
export function IconAction({ label, onClick, children }: IconAction) {
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <button
            type="button"
            onClick={onClick}
            aria-label={label}
            className="flex size-6 shrink-0 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground"
          />
        }
      >
        {children}
      </TooltipTrigger>
      <TooltipContent>{label}</TooltipContent>
    </Tooltip>
  )
}
