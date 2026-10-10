import { useId, type ReactNode } from "react"
import { cn } from "@/lib/utils"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"

interface FooterReadoutProps {
  label: string
  title?: string
  detail: ReactNode
  children: ReactNode
  className?: string
  tooltipClassName?: string
}

export function FooterReadout({
  label,
  title = label,
  detail,
  children,
  className,
  tooltipClassName,
}: FooterReadoutProps) {
  const descriptionId = useId()
  return (
    <Tooltip>
      <TooltipTrigger
        aria-describedby={descriptionId}
        render={
          <button
            type="button"
            className={cn(
              "flex h-8 min-w-0 shrink-0 items-center gap-1.5 whitespace-nowrap rounded-md px-2 tabular-nums outline-none transition-colors hover:bg-accent/50 focus-visible:ring-2 focus-visible:ring-ring",
              className,
            )}
          />
        }
      >
        <span className="sr-only">{label}: </span>
        {children}
      </TooltipTrigger>
      <TooltipContent
        id={descriptionId}
        role="tooltip"
        side="top"
        variant="card"
        className={cn("max-w-80", tooltipClassName)}
      >
        <div className="flex flex-col gap-1.5">
          <span className="font-medium">{title}</span>
          <div className="text-xs text-muted-foreground">{detail}</div>
        </div>
      </TooltipContent>
    </Tooltip>
  )
}
