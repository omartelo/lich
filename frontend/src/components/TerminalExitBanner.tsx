import { RotateCw } from "lucide-react"
import { Button } from "@/components/ui/button"
import { exitNotice, type SessionExit } from "@/lib/terminal/session-exit"

interface TerminalExitBannerProps {
  exit: SessionExit
  /** Spawn the session again, fresh, in the same place. */
  onRestart: () => void
  /** Close the card, through the flow the sidebar's × runs. */
  onClose: () => void
}

// TerminalExitBanner is what a session whose process is gone offers instead of a
// dead prompt. It is anchored to the bottom of the terminal rather than raised
// as a dialog: the scrollback above it is what says why the process is gone, and
// a modal would cover the one thing the user has to read before deciding.
export function TerminalExitBanner({ exit, onRestart, onClose }: TerminalExitBannerProps) {
  return (
    <div className="absolute inset-x-0 bottom-0 z-20 flex items-center gap-3 border-t border-border bg-card px-3 py-2">
      <span className="text-xs text-muted-foreground">{exitNotice(exit)}</span>
      <div className="ml-auto flex items-center gap-1">
        <Button size="xs" variant="ghost" onClick={onRestart}>
          <RotateCw />
          Restart
        </Button>
        <Button size="xs" variant="destructive" onClick={onClose}>
          Close
        </Button>
      </div>
    </div>
  )
}
