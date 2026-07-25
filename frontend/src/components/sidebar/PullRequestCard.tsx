import { GitPullRequestArrow } from "lucide-react"
import { SidebarCard } from "@/components/common/SidebarCard"
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
    <SidebarCard
      icon={GitPullRequestArrow}
      label="Pull request"
      active={active}
      onSelect={onSelect}
      onClose={onClose}
      closeLabel="Close pull request"
    >
      {pr && (
        <span className="flex items-center gap-1.5 text-xs">
          <span className="text-muted-foreground">#{pr.number}</span>
          <span className="text-emerald-500">Open</span>
        </span>
      )}
    </SidebarCard>
  )
}
