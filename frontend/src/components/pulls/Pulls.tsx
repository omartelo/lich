import { useState, type ReactNode } from "react"
import { useNavigate, useParams } from "react-router-dom"
import { toast } from "sonner"
import {
  Check,
  ChevronDown,
  CircleDashed,
  Clock,
  ExternalLink,
  GitBranch,
  GitMerge,
  GitPullRequestArrow,
  RefreshCw,
  X,
  type LucideIcon,
} from "lucide-react"
import { ProjectService, Store, System } from "@/lib/rpc"
import type { ChecksRollup, MergeMethod, PullRequestDetail } from "@/lib/api-types"
import { useProjects } from "@/providers/projects"
import { baseName } from "@/lib/paths"
import { Notice } from "@/components/common/Notice"
import { closePulls } from "@/lib/pulls-card-store"
import { activeTarget, sessionsOf } from "@/lib/session/sessions"
import { useGitStatus } from "@/lib/git/use-git-status"
import { usePullRequestDetail } from "@/lib/pulls/use-pull-request-detail"
import { useInject } from "@/lib/use-inject"
import { cn, errorText } from "@/lib/utils"
import { Markdown } from "@/components/Markdown"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Skeleton } from "@/components/ui/skeleton"
import { PullsChecks } from "./PullsChecks"
import { PullsFiles } from "./PullsFiles"

// Ragged widths, so the body placeholder reads as prose instead of a block.
const BODY_ROWS = ["w-full", "w-11/12", "w-4/5", "w-2/3", "w-5/6", "w-1/2"]

// Pulls is the per-project pull-request screen: it fills the main area on top of
// the persistent terminals (like Settings), showing the active session's branch
// PR — status, body and the full diff — with merge, create and open actions. It
// resolves its path from the route's project id plus the active session (the
// exact-match useActiveSession returns empty on this subroute).
export function Pulls() {
  const { projectId } = useParams()
  const navigate = useNavigate()
  const { projects, sessions, closeSession } = useProjects()
  const projectPath = projects.find((p) => p.id === projectId)?.path ?? ""
  const { sessionId, path } = activeTarget(sessions, projectId ?? null, projectPath)
  const status = useGitStatus(path)
  const branch = status?.branch ?? ""
  const head = status?.head ?? ""
  const { detail, loading, error, refresh } = usePullRequestDetail(path, branch, head)
  const inject = useInject(sessionId)

  // A merged branch leaves its checkout behind — the one cleanup the user would
  // otherwise walk back to the sidebar for. Refuse a dirty worktree: whatever
  // was never committed lives only there, and the sidebar's flow is the one that
  // knows how to confirm discarding it.
  const removeWorktree = async () => {
    if (!projectId || !path) {
      return
    }
    if (await ProjectService.WorktreeDirty(path).catch(() => false)) {
      toast.error("Worktree has uncommitted changes — remove it from the sidebar.")
      return
    }
    // The PTYs living in the checkout must die before git pulls the directory
    // out from under them, and no parked row may survive to offer a resume.
    for (const session of sessionsOf(sessions, projectId).filter((s) => s.path === path)) {
      closeSession(projectId, session.id)
    }
    void Store.PurgeWorktreeSessions(projectId, path)
    closePulls(path)
    navigate(`/projects/${projectId}`)
    try {
      await ProjectService.RemoveWorktree(projectPath, path, false)
      toast.success(`Removed ${baseName(path)}`)
    } catch (err: unknown) {
      toast.error(`Failed to remove worktree: ${errorText(err)}`)
    }
  }

  const onMerged = () => {
    refresh()
    const merged = `Merged #${detail?.number} into ${detail?.baseRefName}`
    // Only a worktree checkout has something to clean up; the project's own
    // directory stays where it is.
    if (path === projectPath) {
      toast.success(merged)
      return
    }
    toast.success(merged, {
      duration: 10_000,
      action: { label: "Remove worktree", onClick: () => void removeWorktree() },
    })
  }

  let body: ReactNode
  if (!path) {
    body = <CentredNotice>No repository</CentredNotice>
  } else if (error) {
    body = <CentredNotice>Couldn’t load the pull request: {error}</CentredNotice>
  } else if (detail) {
    body = (
      <PullRequestView
        path={path}
        head={head}
        detail={detail}
        onRefresh={refresh}
        onMerged={onMerged}
        onInject={inject}
      />
    )
  } else if (loading) {
    body = <PullSkeleton />
  } else {
    body = <EmptyState path={path} branch={branch} onOpened={refresh} />
  }

  return <div className="absolute inset-0 z-10 flex flex-col bg-background">{body}</div>
}

// Looking the pull request up is a gh round-trip, and the screen covers the
// whole app while it runs — a lone "Loading…" in the middle of that much empty
// space reads as a failure. The skeleton holds the header the answer will fill:
// title, actions, status line, tabs, then the body.
function PullSkeleton() {
  return (
    <div aria-busy>
      <div className="border-b border-border px-6 pt-5">
        <div className="flex items-start gap-4">
          <Skeleton className="h-6 w-80 max-w-full" />
          <div className="ml-auto flex flex-none items-center gap-2">
            <Skeleton className="h-8 w-28" />
            <Skeleton className="size-8" />
            <Skeleton className="h-8 w-24" />
          </div>
        </div>
        <div className="mt-4 flex items-center gap-4">
          <Skeleton className="h-3 w-14" />
          <Skeleton className="h-3 w-56" />
          <Skeleton className="h-3 w-36" />
        </div>
        <div className="mt-5 flex gap-6 pb-3">
          <Skeleton className="h-3 w-16" />
          <Skeleton className="h-3 w-24" />
          <Skeleton className="h-3 w-14" />
        </div>
      </div>
      <div className="flex max-w-3xl flex-col gap-3 px-6 py-6">
        {BODY_ROWS.map((width) => (
          <Skeleton key={width} className={`h-3 ${width}`} />
        ))}
      </div>
    </div>
  )
}

// EditState carries a pending "edit commit message" merge: which method to run
// and the message the dialog is editing. null when the dialog is closed.
interface EditState {
  method: MergeMethod
  title: string
  subject: string
  body: string
}

interface PullRequestViewProps {
  path: string
  /** The checkout's HEAD; changing it refetches the diff under the Files tab. */
  head: string
  detail: PullRequestDetail
  /** Re-run the lookup — the manual reload, for what HEAD can't announce. */
  onRefresh: () => void
  /** The merge landed; the screen decides what follows (a toast, the worktree). */
  onMerged: () => void
  onInject: (text: string) => void
}

function PullRequestView({
  path,
  head,
  detail,
  onRefresh,
  onMerged,
  onInject,
}: PullRequestViewProps) {
  const [merging, setMerging] = useState(false)
  const [edit, setEdit] = useState<EditState | null>(null)
  const [tab, setTab] = useState<"overview" | "files" | "checks">("overview")
  const blocked = detail.isDraft
    ? "Pull request is a draft"
    : detail.mergeable === "CONFLICTING"
      ? `Conflicts with ${detail.baseRefName}`
      : null

  const merge = async (method: MergeMethod, subject = "", body = "") => {
    setMerging(true)
    try {
      await ProjectService.MergePullRequest(path, method, subject, body)
      setEdit(null)
      onMerged()
    } catch (err: unknown) {
      toast.error(`Merge failed: ${errorText(err)}`)
    } finally {
      setMerging(false)
    }
  }

  const openEdit = (method: Extract<MergeMethod, "squash" | "merge">) => {
    setEdit(
      method === "squash"
        ? {
            method,
            title: "Squash and merge",
            subject: `${detail.title} (#${detail.number})`,
            body: detail.body,
          }
        : {
            method,
            title: "Create a merge commit",
            subject: `Merge pull request #${detail.number} from ${detail.headRefName}`,
            body: "",
          },
    )
  }

  return (
    <>
      <div className="flex-none border-b border-border px-6 pt-5">
        <div className="flex items-start gap-4">
          <h1 className="min-w-0 flex-1 text-lg font-semibold leading-snug">
            <span className="text-muted-foreground">#{detail.number}</span> {detail.title}
          </h1>
          <div className="flex flex-none items-center gap-2">
            {/* The reason rides on a wrapper: a disabled button takes no pointer
                events, so its own title would never surface. */}
            <span title={blocked ?? undefined}>
              <DropdownMenu>
                <DropdownMenuTrigger
                  render={
                    <Button size="sm" disabled={merging || blocked !== null}>
                      <GitMerge />
                      {merging ? "Merging…" : "Merge"}
                      <ChevronDown />
                    </Button>
                  }
                />
                <DropdownMenuContent align="end">
                  <DropdownMenuItem onClick={() => void merge("squash")}>
                    Squash and merge
                  </DropdownMenuItem>
                  <DropdownMenuItem onClick={() => void merge("merge")}>
                    Create a merge commit
                  </DropdownMenuItem>
                  <DropdownMenuItem onClick={() => void merge("rebase")}>
                    Rebase and merge
                  </DropdownMenuItem>
                  <DropdownMenuSeparator />
                  <DropdownMenuItem onClick={() => openEdit("squash")}>
                    Squash and merge, edit message…
                  </DropdownMenuItem>
                  <DropdownMenuItem onClick={() => openEdit("merge")}>
                    Create a merge commit, edit message…
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            </span>
            {/* HEAD moving refetches on its own; this covers what it can't see —
                a review, a check finishing, a PR opened from the terminal. */}
            <Button variant="ghost" size="sm" onClick={onRefresh} aria-label="Refresh">
              <RefreshCw />
            </Button>
            <Button variant="ghost" size="sm" onClick={() => void System.OpenExternal(detail.url)}>
              <ExternalLink />
              Open
            </Button>
          </div>
        </div>

        <div className="mt-3 flex flex-wrap items-center gap-x-4 gap-y-1.5 text-xs">
          <span
            className={cn(
              "flex items-center gap-1.5 font-medium",
              detail.isDraft ? "text-amber-500" : "text-emerald-500",
            )}
          >
            <GitPullRequestArrow className="size-3.5" />
            {detail.isDraft ? "Draft" : "Open"}
          </span>
          <span className="flex items-center gap-1.5 font-mono text-muted-foreground">
            <GitBranch className="size-3.5" />
            {detail.headRefName} → {detail.baseRefName}
          </span>
          {/* The counter is where the eye lands when CI is red; make it the way
              into the list instead of a dead readout. */}
          {detail.checks.total > 0 ? (
            <button
              type="button"
              onClick={() => setTab("checks")}
              className="rounded-sm transition-opacity hover:opacity-80"
            >
              <ChecksStat checks={detail.checks} />
            </button>
          ) : (
            <ChecksStat checks={detail.checks} />
          )}
          <MergeableStat mergeable={detail.mergeable} base={detail.baseRefName} />
        </div>

        <div role="tablist" className="mt-4 flex gap-1">
          <TabButton active={tab === "overview"} onClick={() => setTab("overview")}>
            Overview
          </TabButton>
          <TabButton active={tab === "files"} onClick={() => setTab("files")}>
            Files changed
            {detail.changedFiles > 0 && (
              <span className="tabular-nums text-muted-foreground">{detail.changedFiles}</span>
            )}
          </TabButton>
          {detail.checks.total > 0 && (
            <TabButton active={tab === "checks"} onClick={() => setTab("checks")}>
              Checks
              <span className="tabular-nums text-muted-foreground">{detail.checks.total}</span>
            </TabButton>
          )}
        </div>
      </div>

      <div role="tabpanel" className="flex-1 overflow-hidden">
        {tab === "files" ? (
          <PullsFiles path={path} head={head} pullRequest={detail.url} onInject={onInject} />
        ) : (
          <div className="h-full overflow-y-auto">
            {tab === "checks" ? (
              <PullsChecks checks={detail.checkRuns} />
            ) : (
              <div className="max-w-3xl px-6 py-5">
                {detail.body.trim() !== "" ? (
                  <Markdown>{detail.body}</Markdown>
                ) : (
                  <p className="text-sm text-muted-foreground">No description.</p>
                )}
              </div>
            )}
          </div>
        )}
      </div>

      {edit && (
        <MergeMessageDialog
          edit={edit}
          merging={merging}
          onChange={setEdit}
          onCancel={() => setEdit(null)}
          onConfirm={() => void merge(edit.method, edit.subject, edit.body)}
        />
      )}
    </>
  )
}

function TabButton({
  active,
  onClick,
  children,
}: {
  active: boolean
  onClick: () => void
  children: ReactNode
}) {
  return (
    <button
      type="button"
      role="tab"
      aria-selected={active}
      onClick={onClick}
      className={cn(
        "-mb-px inline-flex items-center gap-1.5 border-b-2 px-3 py-2 text-sm font-medium transition-colors",
        active
          ? "border-primary text-foreground"
          : "border-transparent text-muted-foreground hover:text-foreground",
      )}
    >
      {children}
    </button>
  )
}

interface MergeMessageDialogProps {
  edit: EditState
  merging: boolean
  onChange: (next: EditState) => void
  onCancel: () => void
  onConfirm: () => void
}

function MergeMessageDialog({
  edit,
  merging,
  onChange,
  onCancel,
  onConfirm,
}: MergeMessageDialogProps) {
  return (
    <Dialog open onOpenChange={(next) => !next && onCancel()}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{edit.title}</DialogTitle>
          <DialogDescription>Edit the commit message, then merge.</DialogDescription>
        </DialogHeader>
        <div className="flex flex-col gap-3">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="merge-subject">Commit message</Label>
            <Input
              id="merge-subject"
              value={edit.subject}
              onChange={(e) => onChange({ ...edit, subject: e.target.value })}
              autoFocus
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="merge-body">Extended description</Label>
            <textarea
              id="merge-body"
              value={edit.body}
              onChange={(e) => onChange({ ...edit, body: e.target.value })}
              rows={6}
              placeholder="Optional"
              className="min-h-24 w-full resize-y rounded-md border border-input bg-transparent px-2.5 py-1.5 text-sm shadow-xs outline-none transition-[color,box-shadow] placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 dark:bg-input/30"
            />
          </div>
        </div>
        <DialogFooter>
          <Button variant="ghost" onClick={onCancel}>
            Cancel
          </Button>
          <Button onClick={onConfirm} disabled={merging || edit.subject.trim() === ""}>
            <GitMerge />
            {merging ? "Merging…" : edit.method === "squash" ? "Squash and merge" : "Merge"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

type Tone = "pass" | "fail" | "pending" | "muted"

const toneClass: Record<Tone, string> = {
  pass: "text-emerald-500",
  fail: "text-destructive",
  pending: "text-amber-500",
  muted: "text-muted-foreground",
}

function Stat({
  icon: Icon,
  tone,
  children,
}: {
  icon: LucideIcon
  tone: Tone
  children: ReactNode
}) {
  return (
    <span className={cn("flex items-center gap-1.5 font-medium", toneClass[tone])}>
      <Icon className="size-3.5" />
      {children}
    </span>
  )
}

function ChecksStat({ checks }: { checks: ChecksRollup }) {
  const { passed, failed, pending, total } = checks
  if (total === 0) {
    return null
  }
  if (failed > 0) {
    return (
      <Stat icon={X} tone="fail">
        {failed} of {total} checks failing
      </Stat>
    )
  }
  if (pending > 0) {
    return (
      <Stat icon={Clock} tone="pending">
        {pending} of {total} checks running
      </Stat>
    )
  }
  return (
    <Stat icon={Check} tone="pass">
      {passed === 1 ? "1 check passed" : `${passed} checks passed`}
    </Stat>
  )
}

function MergeableStat({ mergeable, base }: { mergeable: string; base: string }) {
  if (mergeable === "CONFLICTING") {
    return (
      <Stat icon={X} tone="fail">
        Conflicts with {base}
      </Stat>
    )
  }
  if (mergeable === "MERGEABLE") {
    return (
      <Stat icon={GitMerge} tone="pass">
        Mergeable
      </Stat>
    )
  }
  return (
    <Stat icon={CircleDashed} tone="muted">
      Checking mergeability…
    </Stat>
  )
}

interface EmptyStateProps {
  path: string
  branch: string
  onOpened: () => void
}

function EmptyState({ path, branch, onOpened }: EmptyStateProps) {
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

// The screen-sized variant: the same muted line, centred in the whole area
// rather than sitting at the top of a panel.
function CentredNotice({ children }: { children: ReactNode }) {
  return (
    <div className="flex flex-1 items-center justify-center p-8">
      <Notice className="p-0 text-sm">{children}</Notice>
    </div>
  )
}
