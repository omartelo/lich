import { useState } from "react"
import { toast } from "sonner"
import { Bot, ExternalLink, GitPullRequestArrow } from "lucide-react"
import { Trans } from "@/components/common/Trans"
import { useT } from "@/lib/i18n/i18n"
import { ProjectService } from "@/lib/rpc"
import { Button } from "@/components/ui/button"
import { errorText } from "@/lib/utils"

interface PullsEmptyStateProps {
  path: string
  branch: string
  /** A pull request was opened on GitHub; the screen re-runs its lookup. */
  onOpened: () => void
  /** Hands the writing of the pull request to the checkout's session. */
  onHandOff: () => Promise<void>
  /** Why there is no session to hand it to, null when there is one. */
  handOffBlocked: string | null
}

// What the Pulls screen shows for a checkout whose branch has no open pull
// request: the fact, and the two ways to change it. The agent that made the
// commits can write the pull request, or `gh pr create --web` hands the
// composing off to GitHub; neither grows a form here.
export function PullsEmptyState({
  path,
  branch,
  onOpened,
  onHandOff,
  handOffBlocked,
}: PullsEmptyStateProps) {
  const t = useT()
  const [opening, setOpening] = useState(false)
  const [handingOff, setHandingOff] = useState(false)
  const handOff = async () => {
    setHandingOff(true)
    try {
      await onHandOff()
    } finally {
      setHandingOff(false)
    }
  }
  const openPR = async () => {
    setOpening(true)
    try {
      await ProjectService.CreatePullRequest(path)
      onOpened()
    } catch (err: unknown) {
      toast.error(t("pulls.pullsEmptyState.openFailed", { error: errorText(err) }))
    } finally {
      setOpening(false)
    }
  }
  return (
    <div className="flex flex-1 flex-col items-center justify-center gap-4 p-8 text-center">
      <GitPullRequestArrow className="size-8 text-muted-foreground" />
      <p className="text-sm text-muted-foreground">
        {branch ? (
          <Trans
            k="pulls.pullsEmptyState.noneFor"
            params={{ branch: <span className="font-medium text-foreground">{branch}</span> }}
          />
        ) : (
          t("pulls.pullsEmptyState.none")
        )}
      </p>
      <div className="flex flex-wrap justify-center gap-2">
        <span title={handOffBlocked ?? undefined}>
          <Button
            variant="outline"
            size="sm"
            onClick={() => void handOff()}
            disabled={handingOff || handOffBlocked !== null}
          >
            <Bot />
            {handingOff
              ? t("pulls.pullsEmptyState.openingSession")
              : t("pulls.pullsEmptyState.createWithAgent")}
          </Button>
        </span>
        <Button variant="ghost" size="sm" onClick={() => void openPR()} disabled={opening}>
          <GitPullRequestArrow />
          {opening ? t("pulls.pullsEmptyState.opening") : t("pulls.pullsEmptyState.openOnGitHub")}
          <ExternalLink />
        </Button>
      </div>
    </div>
  )
}
