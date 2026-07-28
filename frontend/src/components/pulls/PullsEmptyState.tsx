import { useState } from "react"
import { toast } from "sonner"
import { ExternalLink, GitPullRequestArrow } from "lucide-react"
import { ProjectService } from "@/lib/rpc"
import { Button } from "@/components/ui/button"
import { errorText } from "@/lib/utils"

interface PullsEmptyStateProps {
  path: string
  branch: string
  /** A pull request was opened on GitHub; the screen re-runs its lookup. */
  onOpened: () => void
}

// What the Pulls screen shows for a checkout whose branch has no open pull
// request: the fact, and the one action that changes it. `gh pr create --web`
// hands the composing off to GitHub rather than growing a form here.
export function PullsEmptyState({ path, branch, onOpened }: PullsEmptyStateProps) {
  const [opening, setOpening] = useState(false)
  const openPR = async () => {
    setOpening(true)
    try {
      await ProjectService.CreatePullRequest(path)
      onOpened()
    } catch (err: unknown) {
      toast.error(`Couldn’t open a pull request: ${errorText(err)}`)
    } finally {
      setOpening(false)
    }
  }
  return (
    <div className="flex flex-1 flex-col items-center justify-center gap-4 p-8 text-center">
      <GitPullRequestArrow className="size-8 text-muted-foreground" />
      <p className="text-sm text-muted-foreground">
        {branch ? (
          <>
            No open pull request for <span className="font-medium text-foreground">{branch}</span>.
          </>
        ) : (
          "No open pull request."
        )}
      </p>
      <Button variant="outline" size="sm" onClick={() => void openPR()} disabled={opening}>
        <GitPullRequestArrow />
        {opening ? "Opening…" : "Open pull request"}
        <ExternalLink />
      </Button>
    </div>
  )
}
