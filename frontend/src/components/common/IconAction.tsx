import type { ReactNode } from "react"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import { cn } from "@/lib/utils"

interface IconActionProps {
  /** Both the tooltip and the accessible name — one string, one meaning. */
  label: string
  onClick: () => void
  children: ReactNode
  /** Makes it a toggle, drawn filled while on. Absent = a one-shot action. */
  pressed?: boolean
}

// A bare icon button for a panel's header strip: no border, no fill at rest,
// its meaning carried by a tooltip. Attach as context, discard changes, collapse
// all, full screen, hide the tree.
export function IconAction({ label, onClick, children, pressed }: IconActionProps) {
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <button
            type="button"
            onClick={onClick}
            aria-label={label}
            aria-pressed={pressed}
            className={cn(
              "flex size-6 shrink-0 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground",
              pressed && "bg-accent text-accent-foreground",
            )}
          />
        }
      >
        {children}
      </TooltipTrigger>
      <TooltipContent>{label}</TooltipContent>
    </Tooltip>
  )
}
