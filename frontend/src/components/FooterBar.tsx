import { useEffect, useState } from "react"
import { useNavigate } from "react-router-dom"
import { openPulls } from "@/lib/pulls-card-store"
import { toast } from "sonner"
import { Code, FileText, GitBranch, Folder, Plus, Diff, GitPullRequestArrow } from "lucide-react"
import { ProjectService, Terminal as TerminalService } from "@/lib/rpc"
import type { DockTab } from "@/components/dock/RightDock"
import { useActiveSession } from "@/lib/useActiveSession"
import { useSessionCwd } from "@/lib/useSessionCwd"
import { useSessionUsage } from "@/lib/useSessionUsage"
import { displayPath } from "@/lib/paths"
import { useGitStatus } from "@/lib/useGitStatus"
import { usePullRequest } from "@/lib/usePullRequest"
import { useSettings } from "@/lib/settings"
import { cn } from "@/lib/utils"
import { DiffStat } from "@/components/DiffStat"
import { ContextRing, contextColor } from "@/components/ContextRing"
import { SessionModel } from "@/components/SessionModel"
import { Separator } from "@/components/ui/separator"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"

const CLOCK_TICK_MS = 10_000

function useNow(): Date {
  const [now, setNow] = useState(() => new Date())
  useEffect(() => {
    const timer = setInterval(() => setNow(new Date()), CLOCK_TICK_MS)
    return () => clearInterval(timer)
  }, [])
  return now
}

const two = (n: number): string => String(n).padStart(2, "0")

interface FooterBarProps {
  dock: DockTab | null
  onDock: (tab: DockTab) => void
}

// FooterBar is the Warp-style status strip. Git segments only render while a
// project is active; everything follows the active session — a worktree session
// shows its checkout's path, branch and diff.
export function FooterBar({ dock, onDock }: FooterBarProps) {
  const navigate = useNavigate()
  const { projectId, sessionId, path: basePath } = useActiveSession()
  // Overlay the backend's live cwd so a `cd` in the terminal moves the footer
  // with it — same source the session card follows. Falls back to the session's
  // static start path until the watcher reports.
  const path = useSessionCwd(sessionId) || basePath
  // Context-window occupancy of the active session, read off its transcript at
  // each turn's end (null until the first turn of a Claude session lands).
  const usage = useSessionUsage(sessionId)
  // The footer context readout is opt-out (Settings › Providers).
  const { showContextUsage } = useSettings()
  const status = useGitStatus(path)
  const pr = usePullRequest(path, status?.branch ?? "", status?.head ?? "")
  const now = useNow()

  const attachFile = async () => {
    try {
      const file = await ProjectService.PickFile()
      if (file && sessionId) {
        void TerminalService.Write(sessionId, `${file} `)
      }
    } catch {
      toast.error("Could not open the file picker")
    }
  }

  // The context-window readout — the ring plus percent, with a detailed
  // tooltip. Null when the user turned it off (Settings › Providers).
  const contextReadout =
    usage && showContextUsage ? (
      <Tooltip>
        <TooltipTrigger
          render={
            <span
              className={cn("flex items-center gap-1.5 tabular-nums", contextColor(usage.percent))}
            />
          }
        >
          <ContextRing percent={usage.percent} />
          {usage.percent}%
        </TooltipTrigger>
        <TooltipContent side="top" className="border border-border bg-card text-foreground">
          <div className="flex flex-col gap-1.5">
            <span className="font-medium">Context window</span>
            <div className={cn("flex items-center gap-2", contextColor(usage.percent))}>
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
      <Tooltip>
        <TooltipTrigger
          render={
            <button
              type="button"
              onClick={() => void attachFile()}
              disabled={!sessionId}
              aria-label="Attach file"
              className="flex items-center justify-center rounded-md p-1 text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground disabled:pointer-events-none disabled:opacity-40"
            />
          }
        >
          <Plus className="size-4" />
        </TooltipTrigger>
        <TooltipContent>Attach file</TooltipContent>
      </Tooltip>
      {path && (
        <Tooltip>
          <TooltipTrigger
            render={
              <button
                type="button"
                onClick={() => onDock("files")}
                aria-pressed={dock === "files"}
                aria-label="Browse code"
                className={`flex items-center justify-center rounded-md p-1 text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground ${
                  dock === "files" ? "bg-accent text-accent-foreground" : ""
                }`}
              />
            }
          >
            <Code className="size-4" />
          </TooltipTrigger>
          <TooltipContent>Browse code</TooltipContent>
        </Tooltip>
      )}
      {status && (
        <Tooltip>
          <TooltipTrigger
            render={
              <button
                type="button"
                onClick={() => onDock("review")}
                aria-pressed={dock === "review"}
                className={`flex items-center gap-1.5 rounded-md px-1.5 py-1 text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground ${
                  dock === "review" ? "bg-accent text-accent-foreground" : ""
                }`}
              />
            }
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
          </TooltipTrigger>
          <TooltipContent>Review changes</TooltipContent>
        </Tooltip>
      )}

      {pr && projectId && (
        <Tooltip>
          <TooltipTrigger
            render={
              <button
                type="button"
                onClick={() => {
                  openPulls(basePath)
                  navigate(`/projects/${projectId}/pulls`)
                }}
                className="flex items-center gap-1.5 rounded-md px-1.5 py-1 text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground"
              />
            }
          >
            <GitPullRequestArrow className="size-3.5" /> PR #{pr.number}
          </TooltipTrigger>
          <TooltipContent>View pull request</TooltipContent>
        </Tooltip>
      )}

      <span className="ml-auto flex items-center gap-4">
        {showContextUsage && <SessionModel sessionId={sessionId} />}
        {contextReadout}
        {contextReadout && (status?.branch || path) && (
          <Separator orientation="vertical" className="h-4" />
        )}
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
