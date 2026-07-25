import { GitPullRequestArrow, X } from "lucide-react"
import { cn } from "@/lib/utils"
import { useGitStatus } from "@/lib/useGitStatus"
import { usePullRequest } from "@/lib/usePullRequest"

interface PullRequestCardProps {
  // The worktree checkout whose branch PR this entry opens.
  path: string
  active: boolean
  onSelect: () => void
  onClose: () => void
}

// PullRequestCard is a worktree group's parked pull-request entry — a peer of
// its session cards, mirroring the Settings card: opening the PR view parks it,
// the X removes it. It opens the full-screen Pulls view for the worktree's
// branch, showing the open PR's number when there is one (and otherwise reading
// as the door to open one, whose create flow lives on the screen's empty state).
export function PullRequestCard({ path, active, onSelect, onClose }: PullRequestCardProps) {
  const git = useGitStatus(path)
  const pr = usePullRequest(path, git?.branch ?? "", git?.head ?? "")
  return (
    <button
      type="button"
      onClick={onSelect}
      className={cn(
        "group relative flex w-full items-center gap-2 rounded-md px-2.5 py-2 pr-7 text-left text-sm font-medium text-foreground transition-colors hover:bg-accent/60",
        active && "bg-accent text-accent-foreground",
      )}
    >
      <GitPullRequestArrow className="size-4 shrink-0 text-muted-foreground" />
      <span>Pull request</span>
      {pr && (
        <span className="flex items-center gap-1.5 text-xs">
          <span className="text-muted-foreground">#{pr.number}</span>
          <span className="text-emerald-500">Open</span>
        </span>
      )}
      <span
        role="button"
        aria-label="Close pull request"
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
