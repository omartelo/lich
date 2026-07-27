import { useMemo, useState, type ReactNode } from "react"
import { useNavigate, useParams } from "react-router-dom"
import { toast } from "sonner"
import {
  Check,
  ChevronDown,
  ExternalLink,
  GitBranch,
  GitMerge,
  GitPullRequestArrow,
  RefreshCw,
  SquareTerminal,
} from "lucide-react"
import { ProjectService, Store, System } from "@/lib/rpc"
import type { MergeMethod, PullRequestDetail } from "@/lib/api-types"
import { useProjects } from "@/providers/projects"
import { baseName } from "@/lib/paths"
import { Notice } from "@/components/common/Notice"
import { closePulls, openPulls } from "@/lib/pulls-card-store"
import { activeTarget, sessionsOf } from "@/lib/session/sessions"
import { queueSetup } from "@/lib/terminal/setup-queue"
import { useGitStatus } from "@/lib/git/use-git-status"
import { useCheckouts } from "@/lib/git/use-checkouts"
import { invalidatePullRequests } from "@/lib/pulls/pull-request-lookup"
import { parsePullsQuery, readPullsSort, type PullsSort } from "@/lib/pulls/pull-request-list"
import { usePullRequestDetail } from "@/lib/pulls/use-pull-request-detail"
import { usePullRequests } from "@/lib/pulls/use-pull-requests"
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
import { PullsCommits } from "./PullsCommits"
import { PullsFiles } from "./PullsFiles"
import { PullsList } from "./PullsList"
import { ChecksStat, MergeableStat, ReviewStat, StateStat } from "./PullsStats"

// Ragged widths, so the body placeholder reads as prose instead of a block.
const BODY_ROWS = ["w-full", "w-11/12", "w-4/5", "w-2/3", "w-5/6", "w-1/2"]

// The merge toast carries the only offer to remove the branch's worktree, so it
// outstays the default: long enough to be read and acted on after the eye goes
// back to the diff, short enough not to sit over the screen.
const CLEANUP_TOAST_MS = 10_000

interface PullsProps {
  /** Show the repository's pull requests in a column beside the one in view.
   * Off, the screen is one pull request and nothing else — the shape a
   * worktree's own card opens, where a list of the repository's others is not
   * what was asked for. */
  list?: boolean
}

// Pulls is the per-project pull-request screen: it fills the main area on top of
// the persistent terminals (like Settings), holding one pull request in full —
// status, body and the full diff — with merge, create, open, and a session on
// the PR's own branch. It resolves its path from the route's project id plus the
// active session (the exact-match useActiveSession returns empty on this
// subroute).
export function Pulls({ list = false }: PullsProps) {
  const { projectId, number } = useParams()
  const navigate = useNavigate()
  const {
    projects,
    sessions,
    closeSession,
    newSession,
    newWorktreeSession,
    reopenWorktreeSession,
    activateSession,
  } = useProjects()
  const projectPath = projects.find((p) => p.id === projectId)?.path ?? ""
  const { sessionId, path } = activeTarget(sessions, projectId ?? null, projectPath)
  const status = useGitStatus(path)
  const branch = status?.branch ?? ""
  const head = status?.head ?? ""
  // No number in the route means the PR of whatever branch this checkout is on
  // — the whole screen without the list, and the default row with it. A number
  // addresses one directly, which only the list can produce.
  const selected = Number(number) || 0
  const { detail, loading, error, refresh } = usePullRequestDetail(path, branch, head, selected)
  // The filter box lives here, not in the column: its `is:` state decides which
  // pull requests gh is asked for, and that is a fetch rather than a filter.
  const [query, setQuery] = useState("")
  const parsedQuery = useMemo(() => parsePullsQuery(query), [query])
  // An empty path is the hook's own "nothing to look up", so the single pull
  // request screen never spends a gh call on a list it does not show.
  const pulls = usePullRequests(list ? projectPath || path : "", parsedQuery.state)
  const { checkouts, refresh: refreshCheckouts } = useCheckouts(projectPath)
  // Where the pull request's own branch already lives, if anywhere. Every
  // "work on this PR" decision hangs off it: whether to create a checkout,
  // whether the button says go rather than open, whether a merge leaves a
  // worktree behind. The project's own directory counts — git refuses a second
  // checkout of a branch just as hard when the project itself holds it.
  const checkedOut = checkouts.find((c) => c.name === detail?.headRefName)
  const inject = useInject(sessionId)
  const [sort, setSort] = useState<PullsSort>(readPullsSort)
  const [opening, setOpening] = useState(false)

  // This screen and the badges around it read the same pull request through two
  // separate lookups, so a change with HEAD standing still — a merge, a PR
  // opened, whatever the reload button was pressed for — has to retire both.
  // The check poll and the focus re-read stay this screen's own.
  const reload = () => {
    invalidatePullRequests()
    refresh()
  }

  // A merged branch leaves its checkout behind — the one cleanup the user would
  // otherwise walk back to the sidebar for. Refuse a dirty worktree: whatever
  // was never committed lives only there, and the sidebar's flow is the one that
  // knows how to confirm discarding it.
  const removeWorktree = async (wtPath: string) => {
    if (!projectId) {
      return
    }
    if (await ProjectService.WorktreeDirty(wtPath).catch(() => false)) {
      toast.error("Worktree has uncommitted changes — remove it from the sidebar.")
      return
    }
    // The PTYs living in the checkout must die before git pulls the directory
    // out from under them, and no parked row may survive to offer a resume.
    for (const session of sessionsOf(sessions, projectId).filter((s) => s.path === wtPath)) {
      closeSession(projectId, session.id)
    }
    void Store.PurgeWorktreeSessions(projectId, wtPath)
    closePulls(wtPath)
    // Only leave the screen when the checkout under it is the one going away.
    if (wtPath === path) {
      navigate(`/projects/${projectId}`)
    }
    try {
      await ProjectService.RemoveWorktree(projectPath, wtPath, false)
      toast.success(`Removed ${baseName(wtPath)}`)
    } catch (err: unknown) {
      toast.error(`Failed to remove worktree: ${errorText(err)}`)
    }
    refreshCheckouts()
  }

  const onMerged = () => {
    reload()
    const merged = `Merged #${detail?.number} into ${detail?.baseRefName}`
    // The offer is about the merged branch's own checkout, which is not
    // necessarily the one this screen is standing in — the list can merge a
    // pull request belonging to a worktree next door, or to none at all. The
    // project's own directory is never offered: it is not a worktree, and git
    // would refuse to remove it.
    const wt = checkedOut?.path !== projectPath ? checkedOut : undefined
    if (!wt) {
      toast.success(merged)
      return
    }
    toast.success(merged, {
      duration: CLEANUP_TOAST_MS,
      action: { label: "Remove worktree", onClick: () => void removeWorktree(wt.path) },
    })
  }

  // Opening a session on a pull request means working on its own head branch.
  // git refuses to check one branch out twice, so a checkout that already holds
  // it is reused rather than recreated — and that includes the project's own
  // directory, which is where a branch usually is.
  const openInSession = async () => {
    if (!projectId || !detail) {
      return
    }
    const existing = checkedOut
    if (existing) {
      const live = sessionsOf(sessions, projectId).find((s) => s.path === existing.path)
      if (live) {
        activateSession(projectId, live.id)
      } else if (existing.path === projectPath) {
        // The project's own checkout is not a worktree: it has no parked
        // session to resume and must never be handed to the worktree flows.
        newSession(projectId)
      } else {
        await reopenWorktreeSession(projectId, existing)
      }
      openPulls(existing.path)
      navigate(`/projects/${projectId}`)
      return
    }
    setOpening(true)
    try {
      const wt = await ProjectService.CreateWorktreeFromPR(projectPath, projectId, detail.number)
      if (!wt) {
        return
      }
      // A fresh checkout is the one moment the project's setup script runs, and
      // the pull request card rides along so the session carries its PR.
      queueSetup(newWorktreeSession(projectId, wt))
      openPulls(wt.path)
      refreshCheckouts()
      navigate(`/projects/${projectId}`)
    } catch (err: unknown) {
      toast.error(`Couldn’t open a session: ${errorText(err)}`)
    } finally {
      setOpening(false)
    }
  }

  const session: SessionAction = {
    label:
      checkedOut && sessionsOf(sessions, projectId ?? "").some((s) => s.path === checkedOut.path)
        ? "Go to session"
        : "Open in Session",
    blocked: detail?.isCrossRepository
      ? "The head branch lives on a fork — its commits could not be pushed back"
      : null,
    busy: opening,
    run: () => void openInSession(),
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
        session={session}
        onRefresh={reload}
        onMerged={onMerged}
        onInject={inject}
      />
    )
  } else if (loading) {
    body = <PullSkeleton />
  } else {
    body = <EmptyState path={path} branch={branch} onOpened={reload} />
  }

  return (
    <div className="absolute inset-0 z-10 flex bg-background">
      {list && (
        <PullsList
          list={pulls.list}
          loading={pulls.loading}
          error={pulls.error}
          selected={selected || (detail?.number ?? 0)}
          onSelect={(picked) => navigate(`/projects/${projectId}/pulls/all/${picked}`)}
          sort={sort}
          onSortChange={setSort}
          query={query}
          onQueryChange={setQuery}
          parsed={parsedQuery}
          checkedOutBranches={new Set(checkouts.map((c) => c.name))}
        />
      )}
      <div className="flex min-w-0 flex-1 flex-col">{body}</div>
    </div>
  )
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

// SessionAction is the header's "work on this pull request" button, resolved by
// the screen: what it should say, whether it can run at all, and what it does.
interface SessionAction {
  /** "Open in Session", or "Go to session" once one is live on the branch. */
  label: string
  /** Why it cannot run (a fork PR), or null when it can. */
  blocked: string | null
  busy: boolean
  run: () => void
}

interface PullRequestViewProps {
  path: string
  /** The checkout's HEAD; changing it refetches the diff under the Files tab. */
  head: string
  detail: PullRequestDetail
  session: SessionAction
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
  session,
  onRefresh,
  onMerged,
  onInject,
}: PullRequestViewProps) {
  const [merging, setMerging] = useState(false)
  const [approving, setApproving] = useState(false)
  const [edit, setEdit] = useState<EditState | null>(null)
  const [tab, setTab] = useState<"overview" | "commits" | "files" | "checks">("overview")
  const commitCount = detail.commits?.length ?? 0
  // A pull request the list reached by number may be over already, in which
  // case there is nothing to merge and gh would refuse anyway.
  const blocked =
    detail.state !== "OPEN"
      ? `Pull request is ${detail.state.toLowerCase()}`
      : detail.isDraft
        ? "Pull request is a draft"
        : detail.mergeable === "CONFLICTING"
          ? `Conflicts with ${detail.baseRefName}`
          : null

  const merge = async (method: MergeMethod, subject = "", body = "") => {
    setMerging(true)
    try {
      await ProjectService.MergePullRequest(path, detail.number, method, subject, body)
      setEdit(null)
      onMerged()
    } catch (err: unknown) {
      toast.error(`Merge failed: ${errorText(err)}`)
    } finally {
      setMerging(false)
    }
  }

  // A draft or a conflicting pull request can still be reviewed — only one that
  // is already over cannot, which is why this does not reuse the merge gate.
  const reviewBlocked =
    detail.state !== "OPEN" ? `Pull request is ${detail.state.toLowerCase()}` : null

  const approve = async () => {
    setApproving(true)
    try {
      await ProjectService.ApprovePullRequest(path, detail.number)
      toast.success(`Approved #${detail.number}`)
      onRefresh()
    } catch (err: unknown) {
      toast.error(`Approve failed: ${errorText(err)}`)
    } finally {
      setApproving(false)
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
            {/* The same wrapper trick as the merge button below: a disabled
                button takes no pointer events, so its own title never shows. */}
            <span title={session.blocked ?? undefined}>
              <Button
                variant="ghost"
                size="sm"
                disabled={session.busy || session.blocked !== null}
                onClick={session.run}
                className="bg-accent/55 text-foreground hover:bg-accent"
              >
                <SquareTerminal />
                {session.busy ? "Opening…" : session.label}
              </Button>
            </span>
            {/* Approving twice is something GitHub allows and nobody means to
                do, so an approved pull request says so instead of offering it
                again — the verdict is in ReviewStat below either way. */}
            <span title={reviewBlocked ?? undefined}>
              <Button
                variant="ghost"
                size="sm"
                disabled={
                  approving || reviewBlocked !== null || detail.reviewDecision === "APPROVED"
                }
                onClick={() => void approve()}
              >
                <Check />
                {approving
                  ? "Approving…"
                  : detail.reviewDecision === "APPROVED"
                    ? "Approved"
                    : "Approve"}
              </Button>
            </span>
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
          <StateStat state={detail.state} isDraft={detail.isDraft} />
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
          <MergeableStat
            mergeable={detail.mergeable}
            base={detail.baseRefName}
            state={detail.state}
          />
          <ReviewStat decision={detail.reviewDecision} />
        </div>

        <div role="tablist" className="mt-4 flex gap-1">
          <TabButton active={tab === "overview"} onClick={() => setTab("overview")}>
            Overview
          </TabButton>
          <TabButton active={tab === "commits"} onClick={() => setTab("commits")}>
            Commits
            {commitCount > 0 && (
              <span className="tabular-nums text-muted-foreground">{commitCount}</span>
            )}
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
          <PullsFiles
            path={path}
            number={detail.number}
            head={head}
            pullRequest={detail.url}
            onInject={onInject}
          />
        ) : (
          <div className="h-full overflow-y-auto">
            {tab === "checks" && <PullsChecks checks={detail.checkRuns} />}
            {tab === "commits" && <PullsCommits commits={detail.commits} />}
            {tab === "overview" && (
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
