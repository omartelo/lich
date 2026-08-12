import { AtSign, GitBranch, GitPullRequestArrow, TriangleAlert } from "lucide-react"
import type { Session } from "@/lib/session/sessions"
import { peerName } from "@/lib/session/peer-name"
import { useSessionCwd } from "@/lib/session/use-session-cwd"
import { useGitStatus } from "@/lib/git/use-git-status"
import { baseReadout } from "@/lib/git/base-status"
import { usePullRequest } from "@/lib/pulls/use-pull-request"
import { DiffStat } from "@/components/DiffStat"
import { TooltipContent } from "@/components/ui/tooltip"

interface SessionTooltipProps {
  session: Session
  // The project's own directory: the fallback for a session with neither a
  // checkout of its own nor a reported cwd.
  path: string
}

// Everything a session card knows, in words — its directory, the name it
// answers to, its branch and how that branch stands against its base. Shared by
// the card and the rail rather than each writing its own: collapsing the
// sidebar takes the card's text away, so this is where the text goes, and two
// versions of it would mean the rail quietly telling you less.
//
// It resolves its own readouts instead of taking them as props. The stores
// behind them are keyed by path and shared (one git poller per repository), so
// the second reader costs a subscription, not a second poll.
export function SessionTooltip({ session, path }: SessionTooltipProps) {
  const liveCwd = useSessionCwd(session.id)
  const shownPath = liveCwd || session.path || path
  const git = useGitStatus(shownPath)
  const pr = usePullRequest(shownPath, git?.branch ?? "", git?.head ?? "")
  const base = baseReadout(git?.base ?? null)
  return (
    <TooltipContent side="right" className="max-w-xs border border-border bg-card text-foreground">
      <div className="flex flex-col gap-1.5">
        <span className="font-medium">{session.label}</span>
        <span className="break-all font-mono text-muted-foreground">{shownPath}</span>
        {/* The name this session answers to when another Claude Code session
            messages it. Built from the start path, not the shown one: a `cd` in
            the terminal never renames a running session. */}
        {session.kind === "claude" && (
          <span className="flex items-center gap-1 font-mono text-muted-foreground">
            <AtSign className="size-3 shrink-0" />
            {peerName(session.path || path, session.id)}
          </span>
        )}
        {git?.branch && (
          <span className="flex flex-wrap items-center gap-2 text-muted-foreground">
            <span className="flex items-center gap-1">
              <GitBranch className="size-3 shrink-0" />
              {git.branch}
            </span>
            {pr && (
              <span className="flex items-center gap-1">
                <GitPullRequestArrow className="size-3 shrink-0" />#{pr.number}
              </span>
            )}
            {git.files > 0 && (
              <span className="flex items-center gap-1.5">
                <DiffStat added={git.added} deleted={git.deleted} />
              </span>
            )}
          </span>
        )}
        {/* The base standing spelled out — the card itself has room for a glyph
            and a number, and this is the only place the branch it measures
            against is named. */}
        {base?.behind && <span className="text-muted-foreground">{base.behind}</span>}
        {base?.conflict && (
          <span className="flex items-center gap-1 text-amber-500">
            <TriangleAlert className="size-3 shrink-0" />
            {base.conflict}
          </span>
        )}
        {base && base.paths.length > 0 && (
          <span className="flex flex-col gap-0.5 pl-4 text-muted-foreground">
            {base.paths.map((file) => (
              <span key={file} className="break-all font-mono">
                {file}
              </span>
            ))}
            {base.more > 0 && <span>+{base.more} more</span>}
          </span>
        )}
      </div>
    </TooltipContent>
  )
}
