import type { ReactNode } from "react"
import { useNavigate } from "react-router-dom"
import { openPulls } from "@/lib/pulls-card-store"
import { toast } from "sonner"
import {
  Code,
  FileText,
  GitBranch,
  Folder,
  Paperclip,
  Diff,
  GitPullRequestArrow,
  Timer,
} from "lucide-react"
import { DropService, Terminal as TerminalService } from "@/lib/rpc"
import type { DockTab } from "@/components/dock/RightDock"
import { useActiveSession } from "@/lib/session/use-active-session"
import { useSessionUsage } from "@/lib/session/use-session-usage"
import { useCostReadout } from "@/lib/use-cost-readout"
import { COST_MISS_REASON, budgetShare, formatCost } from "@/lib/session/session-cost"
import { formatHandsOn, spellHandsOn } from "@/lib/session/hands-on"
import { useRemoteResource } from "@/lib/use-remote-resource"
import { baseName, displayPath } from "@/lib/paths"
import { isWindows } from "@/lib/platform"
import { composeDroppedPaths } from "@/lib/terminal/drop-files"
import { useGitStatus } from "@/lib/git/use-git-status"
import { usePullRequest } from "@/lib/pulls/use-pull-request"
import { useSettings } from "@/providers/settings"
import { useNow } from "@/lib/use-now"
import { cn } from "@/lib/utils"
import { DiffStat } from "./DiffStat"
import { ContextRing, usageColor } from "./ContextRing"
import { PlanQuota } from "./PlanQuota"
import { SessionModel } from "./SessionModel"
import { Separator } from "@/components/ui/separator"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"

const two = (n: number): string => String(n).padStart(2, "0")

interface FooterButtonProps {
  /** Both the tooltip and the accessible name — one string, one meaning. */
  label: string
  onClick: () => void
  /** Set for a button that toggles a dock panel: it reads as pressed while
   * that panel is open. Left off for a plain action. */
  pressed?: boolean
  disabled?: boolean
  /** Roomier padding for a button carrying text beside its glyph. */
  wide?: boolean
  children: ReactNode
}

// One segment of the status strip: a glyph (sometimes with a readout beside
// it), its meaning in a tooltip, and the accent fill that marks the dock panel
// it opens as the one on screen.
function FooterButton({ label, onClick, pressed, disabled, wide, children }: FooterButtonProps) {
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <button
            type="button"
            onClick={onClick}
            disabled={disabled}
            aria-pressed={pressed}
            aria-label={label}
            className={cn(
              "flex items-center rounded-md text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground disabled:pointer-events-none disabled:opacity-40",
              wide ? "gap-1.5 px-1.5 py-1" : "justify-center p-1",
              pressed && "bg-accent text-accent-foreground",
            )}
          />
        }
      >
        {children}
      </TooltipTrigger>
      <TooltipContent>{label}</TooltipContent>
    </Tooltip>
  )
}

interface FooterBarProps {
  dock: DockTab | null
  onDock: (tab: DockTab) => void
}

// FooterBar is the Warp-style status strip. Git segments only render while a
// project is active; everything follows the active session — a worktree session
// shows its checkout's path, branch and diff.
export function FooterBar({ dock, onDock }: FooterBarProps) {
  const navigate = useNavigate()
  const { projectId, sessionId, path, checkout, kind, sandboxed } = useActiveSession()
  // Context-window occupancy of the active session, read off its transcript at
  // each turn's end (null until the first turn of a supported session lands).
  const usage = useSessionUsage(sessionId)
  // The footer context readout is opt-out (Settings › Providers); the cost
  // beside it is opt-in, and off for everyone not billed per token.
  const { showContextUsage, costBudget } = useSettings()
  const showCost = useCostReadout()
  const status = useGitStatus(path)
  const pr = usePullRequest(path, status?.branch ?? "", status?.head ?? "")
  const now = useNow()
  // How long the active session has been worked on. Re-read off the footer's
  // own clock rather than pushed: the figure is minutes, `now` already ticks on
  // the minute, and keying the lookup by it is the whole refresh loop — no
  // event, no store, and a reload paints the number on its first render instead
  // of waiting out a flush.
  const handsOn = useRemoteResource(
    sessionId ? `${sessionId}:${now.getTime()}` : "",
    () => TerminalService.HandsOn(sessionId),
    { empty: 0, resetOn: sessionId },
  )

  // The picker runs on the backend (DropService.Attach), not through
  // ProjectService: a confined session cannot open a file outside its checkout,
  // so the same call that chooses the file also copies it where that session can
  // read it. `checkout` and not `path`: the sandbox is built around the
  // session's spawn directory, which a `cd` does not move.
  const attachFile = async () => {
    if (!sessionId) {
      return
    }
    try {
      const { path: file, copied } = await DropService.Attach(sessionId, checkout, sandboxed)
      if (!file) {
        return
      }
      // Composed the way a drop is: quoted, so a path with a space stays one
      // argument, and bracketed, so the prompt takes it unsent.
      void TerminalService.Write(sessionId, composeDroppedPaths([file], isWindows))
      if (copied) {
        toast.info(`Attached as a copy: ${baseName(file)}`, {
          description:
            "This session is sandboxed, so a file outside its checkout is attached as a copy: edits land on the copy, not on your file, and the copy is deleted when the session closes.",
        })
      }
    } catch (err) {
      // The backend's own sentence when it has one — it names the ceiling a
      // file was refused for, which nothing here could reconstruct.
      toast.error(err instanceof Error ? err.message : "Could not attach the file")
    }
  }

  // What the session has cost, beside the ring. Rendered only when the backend
  // sent a number: on a subscription there is nothing here at all, which is the
  // point of the setting. With a spend ceiling set it takes the same colour ramp
  // as the context ring, so a session running long shows it in the corner of the
  // eye rather than only to whoever reads the figure.
  const costReadout =
    usage && showCost && usage.costUsd !== null ? (
      <Tooltip>
        <TooltipTrigger
          render={
            <span
              className={cn("tabular-nums", usageColor(budgetShare(usage.costUsd, costBudget)))}
            />
          }
        >
          {formatCost(usage.costUsd)}
        </TooltipTrigger>
        <TooltipContent side="top" className="border border-border bg-card text-foreground">
          <div className="flex flex-col gap-1">
            <span className="font-medium">Session cost</span>
            {costBudget > 0 && (
              <span className="font-mono text-xs text-muted-foreground">
                {formatCost(usage.costUsd)} of {formatCost(costBudget)} budget
              </span>
            )}
            <span className="text-xs text-muted-foreground">
              API pricing for every turn this session has run, this conversation and the ones it
              cleared.
            </span>
          </div>
        </TooltipContent>
      </Tooltip>
    ) : null

  // The same slot when there is a number lich cannot produce rather than none to
  // show. An empty corner reads as zero spend, so the absence gets a mark of its
  // own and the tooltip names which of the two standing reasons it is. Muted and
  // never on the budget ramp: nothing is known to be near a ceiling.
  const costMissReadout =
    usage && showCost && usage.costUsd === null && usage.costMiss ? (
      <Tooltip>
        <TooltipTrigger render={<span className="tabular-nums text-muted-foreground" />}>
          $&mdash;
        </TooltipTrigger>
        <TooltipContent side="top" className="border border-border bg-card text-foreground">
          <div className="flex max-w-64 flex-col gap-1">
            <span className="font-medium">Cost unavailable</span>
            <span className="text-xs text-muted-foreground">
              {COST_MISS_REASON[usage.costMiss]}
            </span>
          </div>
        </TooltipContent>
      </Tooltip>
    ) : null

  // How long this session has been worked on, in the same group as the cost so
  // the two read as one fact about the session. Unlike the cost it has no
  // ceiling to be near, so it never takes the amber/red ramp: colouring it
  // would be an opinion about how long is too long, and lich does not have one.
  //
  // The glyph is not decoration. The cost figure is opt-in and Claude-only
  // while this is measured for every session, so the number usually stands
  // alone — and a bare "1h12m" in this strip reads as a quota window, which is
  // drawn two segments to the left.
  const handsOnReadout = formatHandsOn(handsOn.data) ? (
    <Tooltip>
      <TooltipTrigger render={<span className="flex items-center gap-1.5 tabular-nums" />}>
        <Timer className="size-3.5 shrink-0" aria-hidden="true" />
        {formatHandsOn(handsOn.data)}
      </TooltipTrigger>
      <TooltipContent side="top" className="border border-border bg-card text-foreground">
        <div className="flex max-w-64 flex-col gap-1">
          <span className="font-medium">Hands-on time</span>
          <span className="font-mono text-xs text-muted-foreground">
            {spellHandsOn(handsOn.data)}
          </span>
          {/* Keep in sync with handsOnIdleGap in internal/terminal/handson.go. */}
          <span className="text-xs text-muted-foreground">
            How long this session has been worked on — typed at, reporting, or running a turn. A gap
            longer than 15 minutes counts as time away.
          </span>
        </div>
      </TooltipContent>
    </Tooltip>
  ) : null

  // The context-window readout — the ring plus percent, with a detailed
  // tooltip. Null when the user turned it off (Settings › Providers).
  const contextReadout =
    usage && showContextUsage ? (
      <Tooltip>
        <TooltipTrigger
          render={
            <span
              className={cn("flex items-center gap-1.5 tabular-nums", usageColor(usage.percent))}
            />
          }
        >
          <ContextRing percent={usage.percent} />
          {usage.percent}%
        </TooltipTrigger>
        <TooltipContent side="top" className="border border-border bg-card text-foreground">
          <div className="flex flex-col gap-1.5">
            <span className="font-medium">Context window</span>
            <div className={cn("flex items-center gap-2", usageColor(usage.percent))}>
              <span className="h-1.5 w-24 overflow-hidden rounded-full bg-muted">
                <span
                  className="block h-full rounded-full bg-current"
                  style={{ width: `${usage.percent}%` }}
                />
              </span>
              <span className="tabular-nums">{usage.percent}%</span>
            </div>
            <span className="font-mono text-xs text-muted-foreground">
              {usage.tokens.toLocaleString()} / {usage.window.toLocaleString()} tokens
            </span>
          </div>
        </TooltipContent>
      </Tooltip>
    ) : null

  return (
    <footer className="flex h-9 shrink-0 items-center gap-2 border-t border-border bg-sidebar px-3 text-xs text-muted-foreground">
      <FooterButton label="Attach file" onClick={() => void attachFile()} disabled={!sessionId}>
        <Paperclip className="size-4" />
      </FooterButton>
      {path && (
        <FooterButton
          label="Browse code"
          onClick={() => onDock("files")}
          pressed={dock === "files"}
        >
          <Code className="size-4" />
        </FooterButton>
      )}
      {status && (
        <FooterButton
          label="Review changes"
          onClick={() => onDock("review")}
          pressed={dock === "review"}
          wide
        >
          {status.files === 0 ? (
            <>
              <Diff className="size-3.5" /> 0
            </>
          ) : (
            <>
              <FileText className="size-3.5" />
              {status.files}
              <span className="opacity-50">·</span>
              <DiffStat added={status.added} deleted={status.deleted} />
            </>
          )}
        </FooterButton>
      )}

      {pr && projectId && (
        <FooterButton
          label="View pull request"
          onClick={() => {
            openPulls(checkout)
            navigate(`/projects/${projectId}/pulls`)
          }}
          wide
        >
          <GitPullRequestArrow className="size-3.5" /> PR #{pr.number}
        </FooterButton>
      )}

      <span className="ml-auto flex items-center gap-4">
        {showContextUsage && <SessionModel sessionId={sessionId} kind={kind} />}
        <PlanQuota kind={kind} sessionId={sessionId} />
        {(costReadout || costMissReadout || handsOnReadout) && (
          <span className="flex items-center gap-1.5">
            {costReadout ?? costMissReadout}
            {(costReadout || costMissReadout) && handsOnReadout && (
              <span className="opacity-50">·</span>
            )}
            {handsOnReadout}
          </span>
        )}
        {contextReadout}
        {(contextReadout || costReadout || costMissReadout || handsOnReadout) &&
          (status?.branch || path) && <Separator orientation="vertical" className="h-4" />}
        {status?.branch && (
          <span className="flex items-center gap-1">
            <GitBranch className="size-3.5" />
            {status.branch}
          </span>
        )}
        {path && (
          <Tooltip>
            <TooltipTrigger render={<span className="flex items-center gap-1" />}>
              <Folder className="size-3.5" />
              {displayPath(path)}
            </TooltipTrigger>
            <TooltipContent>{path}</TooltipContent>
          </Tooltip>
        )}
        {(status?.branch || path) && <Separator orientation="vertical" className="h-4" />}
        <span>{now.toDateString()}</span>
        <span>{`${two(now.getHours())}:${two(now.getMinutes())}`}</span>
      </span>
    </footer>
  )
}
