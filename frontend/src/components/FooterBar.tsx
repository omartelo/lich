import {useEffect, useState} from "react"
import {useMatch} from "react-router-dom"
import {FileText, GitBranch, Folder, Plus} from "lucide-react"
import {Service as ProjectService} from "../../bindings/github.com/skipodotdev/skipo/internal/project"
import {Service as TerminalService} from "../../bindings/github.com/skipodotdev/skipo/internal/terminal"
import {useProjects} from "@/lib/projects"
import {activeSessionId} from "@/lib/sessions"
import {displayPath} from "@/lib/paths"
import {useGitStatus} from "@/lib/useGitStatus"
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

// FooterBar is the Warp-style status strip: attach-file button and diff counters
// on the left, git branch, project path and clock on the right. Git segments
// only render while a project is active.
export function FooterBar() {
  const {projects, sessions} = useProjects()
  const match = useMatch("/projects/:projectId")
  const projectId = match?.params.projectId ?? null
  const path = projects.find((p) => p.id === projectId)?.path ?? ""
  const sessionId = projectId ? activeSessionId(sessions, projectId) : ""
  const status = useGitStatus(path)
  const now = useNow()

  const attachFile = async () => {
    const file = await ProjectService.PickFile()
    if (file && sessionId) {
      void TerminalService.Write(sessionId, `${file} `)
    }
  }

  return (
    <footer
      className="flex h-9 shrink-0 items-center gap-4 border-t border-border bg-sidebar px-3 text-xs text-muted-foreground">
      <Tooltip>
        <TooltipTrigger
          render={
            <button
              type="button"
              onClick={() => void attachFile()}
              disabled={!sessionId}
              aria-label="Attach file"
              className="flex size-6 items-center justify-center rounded-md border border-border bg-muted text-foreground transition-colors hover:bg-accent hover:text-accent-foreground disabled:pointer-events-none disabled:opacity-40"
            />
          }
        >
          <Plus className="size-4"/>
        </TooltipTrigger>
        <TooltipContent>Attach file</TooltipContent>
      </Tooltip>
      {status && (
        <Tooltip>
          <TooltipTrigger
            render={<span className="flex items-center gap-1.5"/>}
          >
            {status.files === 0 ? (
              <>± 0</>
            ) : (
              <>
                <FileText className="size-3.5"/>
                {status.files}
                <span className="opacity-50">·</span>
                <span className="font-medium text-sky-600 dark:text-sky-400">
                  +{status.added}
                </span>
                <span className="font-medium text-pink-600 dark:text-pink-400">
                  -{status.deleted}
                </span>
              </>
            )}
          </TooltipTrigger>
          <TooltipContent>Uncommitted changes</TooltipContent>
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
        <span>{now.toDateString()}</span>
        <span>{`${two(now.getHours())}:${two(now.getMinutes())}`}</span>
      </span>
    </footer>
  )
}
