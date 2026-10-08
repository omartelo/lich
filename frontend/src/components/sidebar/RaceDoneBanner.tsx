import { Combine } from "lucide-react"
import { Button } from "@/components/ui/button"
import { consolidatorOf } from "@/lib/session/agent-race"
import type { Session } from "@/lib/session/sessions"
import { useSessionStatus } from "@/lib/session/use-session-status"

interface RaceDoneBannerProps {
  projectId: string
  folder: string
  /** The folder's cards, the consolidation among them. */
  sessions: Session[]
  /** How many raced worktrees removing the race would take. */
  rivals: number
  /** Keep the consolidation and remove the race (useWorktreeClose). */
  onRemove: (consolidation: Session) => void
}

// RaceDoneBanner is the race's last step, offered on its folder once the
// consolidation this window opened has finished a turn: by then the agent has
// read the raced worktrees, and what it built lives in a worktree of its own.
// It never removes anything itself; the button asks, like every removal.
export function RaceDoneBanner({
  projectId,
  folder,
  sessions,
  rivals,
  onRemove,
}: RaceDoneBannerProps) {
  const id = consolidatorOf(projectId, folder)
  const consolidation = sessions.find((session) => session.id === id)
  const status = useSessionStatus(consolidation?.id ?? "")
  if (!consolidation || status !== "done" || rivals === 0) {
    return null
  }
  return (
    <div className="flex items-center gap-2 rounded-md bg-accent/60 px-2 py-1.5 text-xs">
      <Combine className="size-3.5 shrink-0 text-muted-foreground" />
      <span className="min-w-0 flex-1 truncate">
        Consolidated in <span className="font-medium">{consolidation.label}</span>
      </span>
      <Button size="xs" variant="destructive" onClick={() => onRemove(consolidation)}>
        Remove the race
      </Button>
    </div>
  )
}
