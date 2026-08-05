import { useMemo, useState, type ReactNode } from "react"
import { useNavigate, useParams } from "react-router-dom"
import { toast } from "sonner"
import { ProjectService, Store } from "@/lib/rpc"
import { useProjects } from "@/providers/projects"
import { baseName } from "@/lib/paths"
import { Notice } from "@/components/common/Notice"
import { closePulls, openPulls } from "@/lib/pulls-card-store"
import { sessionsOf } from "@/lib/session/sessions"
import { useActiveSession } from "@/lib/session/use-active-session"
import { queueSetup } from "@/lib/terminal/setup-queue"
import { useGitStatus } from "@/lib/git/use-git-status"
import { useCheckouts } from "@/lib/git/use-checkouts"
import { invalidatePullRequests } from "@/lib/pulls/pull-request-lookup"
import { parsePullsQuery, readPullsSort, type PullsSort } from "@/lib/pulls/pull-request-list"
import { usePullRequestConversation } from "@/lib/pulls/use-pull-request-conversation"
import { usePullRequestDetail } from "@/lib/pulls/use-pull-request-detail"
import { usePullRequests } from "@/lib/pulls/use-pull-requests"
import { useInject } from "@/lib/use-inject"
import { errorText } from "@/lib/utils"
import { PullRequestView, type SessionAction } from "./PullRequestView"
import { PullSkeleton } from "./PullSkeleton"
import { PullsEmptyState } from "./PullsEmptyState"
import { PullsList } from "./PullsList"

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
// the PR's own branch. It reviews from the active session's checkout, the same
// one the dock beside it browses.
//
// The pull request itself is PullRequestView's; what this owns is everything
// around it — which one is in view, and what an action there means for the
// checkout it lives in.
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
  const { sessionId, path } = useActiveSession()
  const status = useGitStatus(path)
  const branch = status?.branch ?? ""
  const head = status?.head ?? ""
  // No number in the route means the PR of whatever branch this checkout is on
  // — the whole screen without the list, and the default row with it. A number
  // addresses one directly, which only the list can produce.
  const selected = Number(number) || 0
  const { detail, loading, error, refresh } = usePullRequestDetail(path, branch, head, selected)
  // The conversation hangs off the pull request the detail resolved, not off the
  // route: the screen reaches one by number or by branch, and only the answer
  // says which pull request that was.
  const conversation = usePullRequestConversation(path, detail?.number ?? 0, head)
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
    conversation.refresh()
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
    blocked:
      detail?.isCrossRepository && !detail.maintainerCanModify
        ? "The head branch lives on a fork that does not allow edits by maintainers — its commits could not be pushed back"
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
        conversation={conversation.conversation}
        conversationLoading={conversation.loading}
        onConversationRefresh={conversation.refresh}
      />
    )
  } else if (loading) {
    body = <PullSkeleton />
  } else {
    body = <PullsEmptyState path={path} branch={branch} onOpened={reload} />
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

// The screen-sized variant: the same muted line, centred in the whole area
// rather than sitting at the top of a panel.
function CentredNotice({ children }: { children: ReactNode }) {
  return (
    <div className="flex flex-1 items-center justify-center p-8">
      <Notice className="p-0 text-sm">{children}</Notice>
    </div>
  )
}
