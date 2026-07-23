import {useEffect, useState} from "react"
import {toast} from "sonner"
import {Code, FileText, GitBranch, Folder, Plus, Diff, GitPullRequestArrow} from "lucide-react"
import {ProjectService, System, Terminal as TerminalService} from "@/lib/rpc"
import type {DockTab} from "@/components/dock/RightDock"
import {useActiveSession} from "@/lib/useActiveSession"
import {useSessionCwd} from "@/lib/useSessionCwd"
import {displayPath} from "@/lib/paths"
import {useGitStatus} from "@/lib/useGitStatus"
import {usePullRequest} from "@/lib/usePullRequest"
import {DiffStat} from "@/components/DiffStat"
import {Separator} from "@/components/ui/separator"
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip"

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
export function FooterBar({dock, onDock}: FooterBarProps) {
  const {sessionId, path: basePath} = useActiveSession()
  // Overlay the backend's live cwd so a `cd` in the terminal moves the footer
  // with it — same source the session card follows. Falls back to the session's
  // static start path until the watcher reports.
  const path = useSessionCwd(sessionId) || basePath
  const status = useGitStatus(path)
  const pr = usePullRequest(path, status?.branch ?? "")
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

  return (
    <footer
      className="flex h-9 shrink-0 items-center gap-2 border-t border-border bg-sidebar px-3 text-xs text-muted-foreground">
      <Tooltip>
        <TooltipTrigger
          render={
            <button
              type="button"
              onClick={() => void attachFile()}
              disabled={!sessionId}
              aria-label="Attach file"
              className="flex items-center justify-center rounded-md border border-border bg-muted p-1 text-foreground transition-colors hover:bg-accent hover:text-accent-foreground disabled:pointer-events-none disabled:opacity-40"
            />
          }
        >
          <Plus className="size-4"/>
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
                className={`flex items-center justify-center rounded-md border border-border bg-muted p-1 transition-colors hover:bg-accent hover:text-accent-foreground ${
                  dock === "files" ? "bg-accent text-accent-foreground" : ""
                }`}
              />
            }
          >
            <Code className="size-4"/>
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
                className={`flex items-center gap-1.5 rounded-md border border-border bg-muted px-1.5 py-1 transition-colors hover:bg-accent hover:text-accent-foreground ${
                  dock === "review" ? "bg-accent text-accent-foreground" : ""
                }`}
              />
            }
          >
            {status.files === 0 ? (
              <><Diff className="size-3.5"/> 0</>
            ) : (
              <>
                <FileText className="size-3.5"/>
                {status.files}
                <span className="opacity-50">·</span>
                <DiffStat added={status.added} deleted={status.deleted}/>
              </>
            )}
          </TooltipTrigger>
          <TooltipContent>Review changes</TooltipContent>
        </Tooltip>
      )}

      {pr && (
        <Tooltip>
          <TooltipTrigger
            render={
              <button
                type="button"
                onClick={() => void System.OpenExternal(pr.url)}
                className="flex items-center gap-1.5 rounded-md border border-border bg-muted px-1.5 py-1 transition-colors hover:bg-accent hover:text-accent-foreground"
              />
            }
          >
            <GitPullRequestArrow className="size-3.5"/> PR #{pr.number}
          </TooltipTrigger>
          <TooltipContent>Open pull request on GitHub</TooltipContent>
        </Tooltip>
      )}

      <span className="ml-auto flex items-center gap-4">
        {status?.branch && (
          <span className="flex items-center gap-1">
            <GitBranch className="size-3.5"/>
            {status.branch}
          </span>
        )}
        {path && (
          <Tooltip>
            <TooltipTrigger
              render={<span className="flex items-center gap-1"/>}
            >
              <Folder className="size-3.5"/>
              {displayPath(path)}
            </TooltipTrigger>
            <TooltipContent>{path}</TooltipContent>
          </Tooltip>
        )}
        {(status?.branch || path) && (
          <Separator orientation="vertical" className="h-4"/>
        )}
        <span>{now.toDateString()}</span>
        <span>{`${two(now.getHours())}:${two(now.getMinutes())}`}</span>
      </span>
    </footer>
  )
}
