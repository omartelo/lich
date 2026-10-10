import { useSyncExternalStore } from "react"
import { GitPullRequestArrow } from "lucide-react"
import { SidebarCard } from "@/components/common/SidebarCard"
import { useT } from "@/lib/i18n/i18n"
import { useGitStatus } from "@/lib/git/use-git-status"
import { usePullRequest } from "@/lib/pulls/use-pull-request"
import { isPullsOpen, subscribePullsCard } from "@/lib/pulls-card-store"

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
// Nothing is drawn, and nothing looked up, until the checkout's card is parked.
export function PullRequestCard(props: PullRequestCardProps) {
  const open = useSyncExternalStore(subscribePullsCard, () => isPullsOpen(props.path))
  return open ? <ParkedPullRequestCard {...props} /> : null
}

function ParkedPullRequestCard({ path, active, onSelect, onClose }: PullRequestCardProps) {
  const t = useT()
  const git = useGitStatus(path)
  const pr = usePullRequest(path, git?.branch ?? "", git?.head ?? "")
  return (
    <SidebarCard
      icon={GitPullRequestArrow}
      label={t("sidebar.pullRequestCard.label")}
      active={active}
      onSelect={onSelect}
      onClose={onClose}
      closeLabel={t("sidebar.pullRequestCard.close")}
    >
      {pr && (
        <span className="flex items-center gap-1.5 text-xs">
          <span className="text-muted-foreground">#{pr.number}</span>
          <span className="text-tone-pass">{t("sidebar.pullRequestCard.open")}</span>
        </span>
      )}
    </SidebarCard>
  )
}
