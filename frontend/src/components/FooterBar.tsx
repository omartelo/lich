import { Fragment, type ReactNode } from "react"
import { useNavigate } from "react-router-dom"
import { openPulls } from "@/lib/pulls-card-store"
import { toast } from "sonner"
import { Code, FileText, Paperclip, Diff, GitPullRequestArrow } from "lucide-react"
import { DropService, Terminal as TerminalService } from "@/lib/rpc"
import type { DockTab } from "@/components/dock/RightDock"
import { useActiveSession } from "@/lib/session/use-active-session"
import { baseName } from "@/lib/paths"
import { isWindows } from "@/lib/platform"
import { composeDroppedPaths } from "@/lib/terminal/drop-files"
import { useGitStatus } from "@/lib/git/use-git-status"
import { usePullRequest } from "@/lib/pulls/use-pull-request"
import { cn } from "@/lib/utils"
import { useSettings } from "@/providers/settings"
import { useCostReadout } from "@/lib/use-cost-readout"
import { resolveFooterLayout, type FooterItem } from "@/lib/footer-layout"
import { DiffStat } from "./DiffStat"
import { FooterCheckout } from "./FooterCheckout"
import { FooterSession } from "./FooterSession"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"

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
              "flex shrink-0 items-center rounded-md text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground disabled:pointer-events-none disabled:opacity-40",
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
  const { footerLayout, footerVisibility, showContextUsage } = useSettings()
  const showCost = useCostReadout()
  const layout = resolveFooterLayout(footerLayout, footerVisibility, showContextUsage, showCost)
  const { projectId, sessionId, path, cwdHost, checkout, kind, sandboxed } = useActiveSession()
  const status = useGitStatus(path)
  const pr = usePullRequest(path, status?.branch ?? "", status?.head ?? "")
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

  const controls: Partial<Record<FooterItem, ReactNode>> = {
    attach: (
      <FooterButton label="Attach file" onClick={() => void attachFile()} disabled={!sessionId}>
        <Paperclip className="size-4" />
      </FooterButton>
    ),
    files: path && (
      <FooterButton label="Browse code" onClick={() => onDock("files")} pressed={dock === "files"}>
        <Code className="size-4" />
      </FooterButton>
    ),
    changes: status && (
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
    ),
    pr: pr && projectId && (
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
    ),
    checkout: path && <FooterCheckout path={path} branch={status?.branch ?? ""} />,
    path: path && (
      <FooterCheckout path={path} branch={status?.branch ?? ""} display="path" host={cwdHost} />
    ),
  }
  return (
    <footer className="flex min-h-9 min-w-0 shrink-0 flex-wrap items-center justify-between gap-x-4 gap-y-1 border-t border-border bg-sidebar px-3 py-1 text-xs text-muted-foreground">
      {(["left", "right"] as const).map((side) => (
        <section
          key={side}
          aria-label={side === "left" ? "Left footer" : "Right footer"}
          data-footer-side={side}
          className={cn(
            "flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1",
            side === "right" && "ml-auto justify-end",
          )}
        >
          {layout[side].map((id) => (
            <Fragment key={id}>
              {id in controls ? (
                controls[id]
              ) : (
                <FooterSession sessionId={sessionId} kind={kind} item={id} />
              )}
            </Fragment>
          ))}
        </section>
      ))}
    </footer>
  )
}
