import { Trophy } from "lucide-react"
import { ContextMenuItem } from "@/components/ui/context-menu"
import { count } from "@/lib/utils"

interface SessionKeepItemProps {
  /** The folder the card is filed in. */
  folder: string
  /** How many other worktrees in that folder the keep removes (raceRivals). */
  rivals: number
  onKeep: () => void
}

// "Keep this, remove the others" on a card filed with other worktrees: how a
// race ends, from the card that won it. Absent where there is nothing to remove,
// so a folder of one checkout never offers it.
export function SessionKeepItem({ folder, rivals, onKeep }: SessionKeepItemProps) {
  if (rivals === 0) {
    return null
  }
  return (
    <ContextMenuItem onClick={onKeep}>
      <Trophy />
      <span className="flex flex-col items-start">
        Keep this, remove the others
        <span className="text-xs">
          {count(rivals, "other worktree")} in {folder}
        </span>
      </span>
    </ContextMenuItem>
  )
}
