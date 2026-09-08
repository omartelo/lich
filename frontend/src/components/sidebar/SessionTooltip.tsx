import {
  ArrowLeft,
  ArrowRight,
  Clock,
  GitBranch,
  GitPullRequestArrow,
  Play,
  Shield,
  ShieldOff,
  TriangleAlert,
} from "lucide-react"
import { unknownCwd } from "@/lib/paths"
import { sandboxDrift } from "@/lib/providers-store"
import type { Session } from "@/lib/session/sessions"
import { useSandboxRung } from "@/lib/use-sandbox-rung"
import { useSessionCwd } from "@/lib/session/use-session-cwd"
import { useSessionRelay } from "@/lib/session/use-session-relay"
import { scheduledFor } from "@/lib/session/schedule"
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
  // The project this session sits in — the scope its provider's sandbox rung is
  // read in. The rail is the reason it is a prop: collapsed, this tooltip is the
  // only text a session has, so it cannot be the half that stops explaining.
  projectId: string
}

// Everything a session card knows, in words — its directory, its branch and how
// that branch stands against its base. Shared by the card and the rail rather
// than each writing its own: collapsing the sidebar takes the card's text away,
// so this is where the text goes, and two versions of it would mean the rail
// quietly telling you less.
//
// The peer-roster name is deliberately not among them: it addresses a session
// from inside another one, which is not something the person reading this
// tooltip is doing. `/list-agents` and lich's own list_sessions are where an
// agent meets it.
//
// It resolves its own readouts instead of taking them as props. The stores
// behind them are keyed by path and shared (one git poller per repository), so
// the second reader costs a subscription, not a second poll.
export function SessionTooltip({ session, path, projectId }: SessionTooltipProps) {
  const { cwd: liveCwd, host: cwdHost } = useSessionCwd(session.id)
  const relay = useSessionRelay(session.id)
  const shownPath = liveCwd || session.path || path
  const git = useGitStatus(shownPath)
  const pr = usePullRequest(shownPath, git?.branch ?? "", git?.head ?? "")
  const base = baseReadout(git?.base ?? null)
  const rung = useSandboxRung(session.kind, projectId)
  const confined = session.sandboxed ?? false
  const skippedLinks = session.sandboxSkippedLinks ?? []
  const drift = rung === null ? "" : sandboxDrift(rung, !!session.path, confined)
  return (
    <TooltipContent side="right" className="max-w-xs border border-border bg-card text-foreground">
      <div className="flex flex-col gap-1.5">
        <span className="font-medium">{session.label}</span>
        <span className="break-all font-mono text-muted-foreground">
          {cwdHost ? unknownCwd(cwdHost) : shownPath}
        </span>
        {/* The open request, and the ticket it runs on. The card already draws
            the arrow and the peer; the number is the part that exists nowhere
            else a person can read it — it is typed once, into the target's
            prompt, and an agent whose context was compacted past that message
            has no way back to it. The reply line is drawn only on the side that
            owes the answer, because it is what you hand back to a session that
            lost the instruction; the side that is waiting has nothing to run. */}
        {relay && (
          <span className="flex flex-col gap-0.5">
            <span className="flex items-center gap-1.5 text-muted-foreground">
              {relay.direction === "out" ? (
                <ArrowRight className="size-3 shrink-0" />
              ) : (
                <ArrowLeft className="size-3 shrink-0" />
              )}
              <span>
                {relay.direction === "out" ? "Waiting on " : "Answering "}
                {relay.peer ? (
                  <span className="font-medium text-foreground">{relay.peer}</span>
                ) : (
                  <span className="italic">the command line</span>
                )}
              </span>
            </span>
            {relay.ticket && <span className="font-mono tabular-nums">ticket {relay.ticket}</span>}
            {relay.ticket && relay.direction === "in" && (
              <span className="break-all font-mono text-muted-foreground">
                {`lich reply ${relay.ticket} "…"`}
              </span>
            )}
          </span>
        )}
        {/* The prompt parked on this session. The card counts down to it; the
            day and the wording are here, because "in 2d" is not a time anyone
            can plan around and the prompt itself is the part the user has to
            recognise to know whether to leave it there. */}
        {session.scheduledAt && (
          <span className="flex flex-col gap-0.5">
            <span className="flex items-center gap-1.5 text-muted-foreground">
              <Clock className="size-3 shrink-0" />
              <span>
                Scheduled for{" "}
                <span className="font-medium text-foreground">
                  {scheduledFor(session.scheduledAt, new Date())}
                </span>
              </span>
            </span>
            <span className="line-clamp-2 text-muted-foreground italic">
              {session.scheduledPrompt}
            </span>
          </span>
        )}
        {/* The command this terminal opens into. The card's label already says
            it while the label is still automatic — this is the only place it is
            named once the user has renamed the card. */}
        {session.entrypoint && (
          <span className="flex items-center gap-1.5 text-muted-foreground">
            <Play className="size-3 shrink-0" />
            <span className="break-all font-mono">{session.entrypoint}</span>
          </span>
        )}
        {/* What the shield on the card means. Directly under the path, because
            what it says is about that path: the sandbox is the difference
            between a session that can write anywhere and one that can write
            here. The last line is the part nothing else says — the answer was
            taken when the session opened, and moving the rung in Settings will
            not move this card.

            An unconfined session is silent here unless the rung has since moved
            past it, which is the one time its lack of a shield is news.

            The last line names what the sandbox left out of the private home
            for being a symlink. Every path it binds is taken as it is on disk
            and a link is skipped, so a ~/.gitconfig kept in a dotfiles
            repository is not in there — an absence the session would otherwise
            only meet as a command that behaves differently inside the sandbox
            than outside it. Nothing skipped draws nothing. */}
        {(confined || drift) && (
          <span className="flex flex-col gap-0.5">
            <span className="flex items-center gap-1.5">
              {confined ? (
                <Shield className="size-3 shrink-0" />
              ) : (
                <ShieldOff className="size-3 shrink-0" />
              )}
              {confined ? "Sandboxed" : "Not sandboxed"}
            </span>
            <span className="text-muted-foreground">
              {confined
                ? "Empty home, machine read-only, writes only in this checkout. "
                : "This session runs on the machine. "}
              {drift
                ? "Opened before the sandbox setting changed; reopen the session to apply it."
                : "Set when the session opened; reopen it to change."}
            </span>
            {skippedLinks.length > 0 && (
              <span className="text-muted-foreground">
                Not mounted (symlinks): {skippedLinks.join(", ")}
              </span>
            )}
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
