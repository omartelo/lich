import { X } from "lucide-react"
import type { MouseEvent } from "react"
import { cn } from "@/lib/utils"

interface CloseButtonProps {
  /** Announced to screen readers — say what closes, e.g. "Close settings". */
  label: string
  onClick: (event: MouseEvent<HTMLSpanElement>) => void
  /** Where it sits in its row; the parent owns the positioning. */
  className?: string
}

// The × that appears on hover over a card or tab. A span, not a button: every
// caller nests it inside a <button> or an <a>, and a button inside a button is
// invalid HTML the browser un-nests. The caller's own handler stops the click
// from reaching that parent — how it does so differs (stopPropagation on a
// button, preventDefault on a NavLink), so it stays with the caller.
//
// The parent must carry `group` for the hover reveal to fire.
export function CloseButton({ label, onClick, className }: CloseButtonProps) {
  return (
    <span
      role="button"
      aria-label={label}
      onClick={onClick}
      className={cn(
        "flex size-4 shrink-0 items-center justify-center rounded opacity-0 transition-opacity hover:bg-foreground/15 group-hover:opacity-100",
        className,
      )}
    >
      <X className="size-3" />
    </span>
  )
}
