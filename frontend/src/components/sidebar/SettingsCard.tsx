import { Settings, X } from "lucide-react"
import { cn } from "@/lib/utils"

interface SettingsCardProps {
  active: boolean
  onSelect: () => void
  onClose: () => void
}

// SettingsCard is the project's Settings entry in the session list: it appears
// when settings is opened for the project and stays parked (inactive) while the
// user works in a terminal, mirroring SessionCard's shape so it reads as a peer
// of the sessions rather than a separate control.
export function SettingsCard({ active, onSelect, onClose }: SettingsCardProps) {
  return (
    <button
      type="button"
      onClick={onSelect}
      className={cn(
        "group relative flex w-full items-center gap-2 rounded-md px-2.5 py-2 text-left text-sm font-medium text-foreground transition-colors hover:bg-accent/60",
        active && "bg-accent text-accent-foreground",
      )}
    >
      <Settings className="size-4 shrink-0 text-muted-foreground" />
      <span>Settings</span>
      <span
        role="button"
        aria-label="Close settings"
        onClick={(event) => {
          event.stopPropagation()
          onClose()
        }}
        className="absolute right-2 top-1/2 flex size-4 -translate-y-1/2 items-center justify-center rounded opacity-0 transition-opacity hover:bg-foreground/15 group-hover:opacity-100"
      >
        <X className="size-3" />
      </span>
    </button>
  )
}
