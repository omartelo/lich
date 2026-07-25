import type { LucideIcon } from "lucide-react"
import type { ReactNode } from "react"
import { CloseButton } from "./CloseButton"
import { cn } from "@/lib/utils"

interface SidebarCardProps {
  icon: LucideIcon
  label: string
  active: boolean
  onSelect: () => void
  onClose: () => void
  /** What the × announces, e.g. "Close settings". */
  closeLabel: string
  /** Trailing detail beside the label — the pull request's number and state. */
  children?: ReactNode
}

// A parked screen's entry in the session list: Settings, Pull request. It reads
// as a peer of the session cards rather than a separate control, so it borrows
// their row shape — borderless, hover fill, active fill, × on hover. Opening the
// screen parks the card; the × removes it.
export function SidebarCard({
  icon: Icon,
  label,
  active,
  onSelect,
  onClose,
  closeLabel,
  children,
}: SidebarCardProps) {
  return (
    <button
      type="button"
      onClick={onSelect}
      className={cn(
        "group relative flex w-full items-center gap-2 rounded-md px-2.5 py-2 pr-7 text-left text-sm font-medium text-foreground transition-colors hover:bg-accent/60",
        active && "bg-accent text-accent-foreground",
      )}
    >
      <Icon className="size-4 shrink-0 text-muted-foreground" />
      <span>{label}</span>
      {children}
      <CloseButton
        label={closeLabel}
        onClick={(event) => {
          event.stopPropagation()
          onClose()
        }}
        className="absolute right-2 top-1/2 -translate-y-1/2"
      />
    </button>
  )
}
