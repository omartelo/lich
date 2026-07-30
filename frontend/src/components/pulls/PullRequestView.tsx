import { useState, useSyncExternalStore, type ReactNode } from "react"
import { toast } from "sonner"
import {
  Check,
  ChevronDown,
  ExternalLink,
  GitBranch,
  GitMerge,
  MessageSquare,
  RefreshCw,
  SquareTerminal,
} from "lucide-react"
import { ProjectService, System } from "@/lib/rpc"
import type {
  MergeMethod,
  PullRequestConversation,
  PullRequestDetail,
  ReviewEvent,
} from "@/lib/api-types"
import {
  clearPendingReview,
  pendingReview,
  setReviewBody,
  subscribePendingReview,
} from "@/lib/pulls/pending-review-store"
import { conversationCount, conversationTimeline } from "@/lib/pulls/conversation-timeline"
import { cn, errorText } from "@/lib/utils"
import { Markdown } from "@/components/Markdown"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { MergeMessageDialog, mergeEditFor, type MergeEdit } from "./MergeMessageDialog"
import { PullsChecks } from "./PullsChecks"
import { PullsCommits } from "./PullsCommits"
import { PullsConversation } from "./PullsConversation"
import { PullsFiles } from "./PullsFiles"
import { ChecksStat, MergeableStat, ReviewStat, StateStat } from "./PullsStats"
import type { ThreadActions } from "./ReviewThread"
import { SubmitReviewDialog } from "./SubmitReviewDialog"

// SessionAction is the header's "work on this pull request" button, resolved by
// the screen: what it should say, whether it can run at all, and what it does.
export interface SessionAction {
  /** "Open in Session", or "Go to session" once one is live on the branch. */
  label: string
  /** Why it cannot run (a fork PR), or null when it can. */
  blocked: string | null
  busy: boolean
  run: () => void
}

type Tab = "overview" | "commits" | "files" | "conversation" | "checks"

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
  /** What has been said about this pull request, and how to re-read it after a
   * reply, a resolve or a submitted review. */
  conversation: PullRequestConversation | null
  conversationLoading: boolean
  onConversationRefresh: () => void
}

// PullRequestView is one pull request in full: the header (title, the actions
// that change it, its status line) over the four tabs that read it. It owns the
// actions' in-flight state and nothing else — which pull request to show, and
// what a merge means for the worktree it lives in, stay the screen's.
export function PullRequestView({
  path,
  head,
  detail,
  session,
  onRefresh,
  onMerged,
  onInject,
  conversation,
  conversationLoading,
  onConversationRefresh,
}: PullRequestViewProps) {
  const [merging, setMerging] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [verdict, setVerdict] = useState<ReviewEvent | null>(null)
  const [edit, setEdit] = useState<MergeEdit | null>(null)
  const [tab, setTab] = useState<Tab>("overview")
  const commitCount = detail.commits?.length ?? 0
  const review = useSyncExternalStore(subscribePendingReview, () => pendingReview(detail.url))
  const pending = review.comments.length
  const talk = conversationCount(conversationTimeline(conversation))
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

  // Every write to a thread re-reads the conversation rather than patching what
  // is on screen: GitHub decides what a reply and a resolve actually did, and a
  // hand-rolled optimistic copy is a second source of truth for no gain.
  const actions: ThreadActions = {
    reply: async (commentID, body) => {
      await ProjectService.ReplyToReviewThread(path, detail.number, commentID, body)
      onConversationRefresh()
    },
    resolve: async (threadID, resolved) => {
      await ProjectService.ResolveReviewThread(path, threadID, resolved)
      onConversationRefresh()
    },
  }

  const comment = async (body: string) => {
    await ProjectService.CommentOnPullRequest(path, detail.number, body)
    onConversationRefresh()
  }

  // The whole review in one call: the verdict, the summary, and every comment
  // held back since the first one was written.
  const submitReview = async (event: ReviewEvent) => {
    setSubmitting(true)
    try {
      await ProjectService.SubmitReview(
        path,
        detail.number,
        event,
        review.body,
        conversation?.headRefOid ?? "",
        review.comments,
      )
      clearPendingReview(detail.url)
      setVerdict(null)
      toast.success(
        event === "approve"
          ? `Approved #${detail.number}`
          : `Review sent on #${detail.number}${pending > 0 ? ` — ${pending} comments` : ""}`,
      )
      onRefresh()
      onConversationRefresh()
    } catch (err: unknown) {
      toast.error(`Review failed: ${errorText(err)}`)
    } finally {
      setSubmitting(false)
    }
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
            {/* With nothing written, reviewing is the one-click approval it has
                always been. The moment a comment is waiting, the same control
                becomes the way to send the review it belongs to — and says how
                many are riding on it. Approving twice is something GitHub allows
                and nobody means to do, so an approved pull request says so
                instead of offering it again. */}
            <span title={reviewBlocked ?? undefined}>
              {pending === 0 ? (
                <Button
                  variant="ghost"
                  size="sm"
                  disabled={
                    submitting || reviewBlocked !== null || detail.reviewDecision === "APPROVED"
                  }
                  onClick={() => void submitReview("approve")}
                >
                  <Check />
                  {submitting
                    ? "Approving…"
                    : detail.reviewDecision === "APPROVED"
                      ? "Approved"
                      : "Approve"}
                </Button>
              ) : (
                <DropdownMenu>
                  <DropdownMenuTrigger
                    render={
                      <Button size="sm" variant="ghost" disabled={reviewBlocked !== null}>
                        <MessageSquare />
                        Submit review
                        <span className="tabular-nums text-muted-foreground">{pending}</span>
                        <ChevronDown />
                      </Button>
                    }
                  />
                  <DropdownMenuContent align="end">
                    <DropdownMenuItem onClick={() => setVerdict("comment")}>
                      Comment
                    </DropdownMenuItem>
                    <DropdownMenuItem onClick={() => setVerdict("approve")}>
                      Approve
                    </DropdownMenuItem>
                    <DropdownMenuItem onClick={() => setVerdict("request_changes")}>
                      Request changes
                    </DropdownMenuItem>
                    <DropdownMenuSeparator />
                    <DropdownMenuItem onClick={() => clearPendingReview(detail.url)}>
                      Discard {pending} pending {pending === 1 ? "comment" : "comments"}
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              )}
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
                  <DropdownMenuItem onClick={() => setEdit(mergeEditFor("squash", detail))}>
                    Squash and merge, edit message…
                  </DropdownMenuItem>
                  <DropdownMenuItem onClick={() => setEdit(mergeEditFor("merge", detail))}>
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
          <TabButton active={tab === "conversation"} onClick={() => setTab("conversation")}>
            Conversation
            {talk > 0 && <span className="tabular-nums text-muted-foreground">{talk}</span>}
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
            threads={conversation?.threads ?? null}
            actions={actions}
          />
        ) : (
          <div className="h-full overflow-y-auto">
            {tab === "checks" && <PullsChecks checks={detail.checkRuns} />}
            {tab === "commits" && <PullsCommits commits={detail.commits} />}
            {tab === "conversation" && (
              <PullsConversation
                conversation={conversation}
                loading={conversationLoading}
                actions={actions}
                onComment={comment}
              />
            )}
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

      {verdict && (
        <SubmitReviewDialog
          event={verdict}
          review={review}
          submitting={submitting}
          onBodyChange={(body) => setReviewBody(detail.url, body)}
          onCancel={() => setVerdict(null)}
          onSubmit={() => void submitReview(verdict)}
        />
      )}

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
