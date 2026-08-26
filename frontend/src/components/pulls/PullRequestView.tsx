import { useMemo, useState, useSyncExternalStore, type ReactNode } from "react"
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
  X,
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
import {
  allowedMergeMethods,
  canAdminOverride,
  mergeBlockedReason,
  mergeRuleNote,
} from "@/lib/pulls/merge-gate"
import { readPullsTab, writePullsTab, type PullsTab } from "@/lib/pulls/pulls-prefs"
import { useBranchRules } from "@/lib/pulls/use-branch-rules"
import { cn, errorText } from "@/lib/utils"
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
import { PullsOverview } from "./PullsOverview"
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

const MERGE_LABELS: Record<MergeMethod, string> = {
  squash: "Squash and merge",
  merge: "Create a merge commit",
  rebase: "Rebase and merge",
}

// What the hover says when nothing blocks the merge but CI is red. GitHub only
// reports a failing check as blocking where a rule requires it; everywhere else
// the merge is offered and the call is the reader's, which is the whole reason
// this has to be said out loud.
function failingChecks(failed: number): string | undefined {
  if (failed === 0) {
    return undefined
  }
  const checks = failed === 1 ? "check is" : "checks are"
  return `${failed} ${checks} failing — GitHub will still merge this`
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
  /** Write into the session's terminal; false when no session took it. */
  onInject: (text: string) => boolean
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
  // Which tab is a habit, not a fact about this pull request: a reviewer who
  // lives in Files changed should land there on the next one too, and on the
  // same one after a detour into another project.
  const [tab, setTab] = useState<PullsTab>(readPullsTab)
  const showTab = (next: PullsTab) => {
    writePullsTab(next)
    setTab(next)
  }
  // Read here rather than threaded down from the screen: it is keyed by the
  // base branch, which only exists once the detail has resolved — and this is
  // the one place that is guaranteed.
  const rules = useBranchRules(path, detail.baseRefName)
  const commitCount = detail.commits?.length ?? 0
  const review = useSyncExternalStore(subscribePendingReview, () => pendingReview(detail.url))
  const pending = review.comments.length
  const talk = conversationCount(conversationTimeline(conversation))
  const blocked = mergeBlockedReason(detail)
  const methods = allowedMergeMethods(rules)
  const editableMethods = methods.filter((method) => method !== "rebase")
  const ruleNote = mergeRuleNote(detail, rules)
  const bypass = canAdminOverride(detail, rules)

  const merge = async (method: MergeMethod, subject = "", body = "", admin = false) => {
    setMerging(true)
    try {
      await ProjectService.MergePullRequest(path, detail.number, method, subject, body, admin)
      setEdit(null)
      onMerged()
    } catch (err: unknown) {
      // gh's refusal names a kind of cause, never the rule behind it; the note
      // is the only thing on screen that can, so it rides along when there is
      // one to say.
      toast.error([`Merge failed: ${errorText(err)}`, ruleNote].filter(Boolean).join(" "))
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
  //
  // Memoised because the Files tab hangs its per-file review objects off this
  // one, and those ride the identity of each diff's CodeMirror decorations.
  const actions: ThreadActions = useMemo(
    () => ({
      reply: async (commentID, body) => {
        await ProjectService.ReplyToReviewThread(path, detail.number, commentID, body)
        onConversationRefresh()
      },
      resolve: async (threadID, resolved) => {
        await ProjectService.ResolveReviewThread(path, threadID, resolved)
        onConversationRefresh()
      },
    }),
    [path, detail.number, onConversationRefresh],
  )

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
                events, so its own title would never surface. With nothing
                blocking, the same spot carries what would make you think twice
                — red CI that no rule requires, which GitHub merges over
                without a word. */}
            <span title={blocked ?? ruleNote ?? failingChecks(detail.checks.failed)}>
              <DropdownMenu>
                <DropdownMenuTrigger
                  render={
                    <Button size="sm" disabled={merging || (blocked !== null && !bypass)}>
                      <GitMerge />
                      {merging ? "Merging…" : "Merge"}
                      {/* The checks readout says this too, a line above — but
                          the decision is taken here, and a count beside the
                          button is the only place it cannot be missed. */}
                      {detail.checks.failed > 0 && (
                        <span className="flex items-center gap-0.5">
                          <X className="size-3.5" />
                          <span className="tabular-nums">{detail.checks.failed}</span>
                        </span>
                      )}
                      <ChevronDown />
                    </Button>
                  }
                />
                {/* Every item bypasses, or none does — never a menu offering
                    both. Once GitHub answers BLOCKED or BEHIND gh refuses a
                    plain merge from the client without ever calling GitHub, so
                    where the bypass is available the ordinary items are already
                    dead and keeping them would put the click that cannot work
                    beside the one that can. Without the privilege the dead
                    click stays, on purpose: it is what carries gh's refusal and
                    the rule note back (merge-gate.ts).

                    The consequence to know: BLOCKED is what GitHub answers for
                    any governed base branch, rules met or not, so an
                    administrator's ordinary approved merge goes out with
                    --admin here and GitHub records it as an override. gh leaves
                    no other way through. */}
                <DropdownMenuContent align="end">
                  {methods.map((method) => (
                    <DropdownMenuItem
                      key={method}
                      onClick={() => void merge(method, "", "", bypass)}
                    >
                      {MERGE_LABELS[method]}
                      {bypass && ", bypass rules"}
                    </DropdownMenuItem>
                  ))}
                  {/* Rebase replays the branch's own commits, so gh takes no
                      message for it and it has no "edit message" twin. */}
                  {editableMethods.length > 0 && <DropdownMenuSeparator />}
                  {editableMethods.map((method) => (
                    <DropdownMenuItem
                      key={`${method}-edit`}
                      onClick={() => setEdit(mergeEditFor(method, detail))}
                    >
                      {MERGE_LABELS[method]}, edit message…
                      {bypass && " (bypasses rules)"}
                    </DropdownMenuItem>
                  ))}
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
              onClick={() => showTab("checks")}
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
          <TabButton active={tab === "overview"} onClick={() => showTab("overview")}>
            Overview
          </TabButton>
          <TabButton active={tab === "commits"} onClick={() => showTab("commits")}>
            Commits
            {commitCount > 0 && (
              <span className="tabular-nums text-muted-foreground">{commitCount}</span>
            )}
          </TabButton>
          <TabButton active={tab === "files"} onClick={() => showTab("files")}>
            Files changed
            {detail.changedFiles > 0 && (
              <span className="tabular-nums text-muted-foreground">{detail.changedFiles}</span>
            )}
          </TabButton>
          <TabButton active={tab === "conversation"} onClick={() => showTab("conversation")}>
            Conversation
            {talk > 0 && <span className="tabular-nums text-muted-foreground">{talk}</span>}
          </TabButton>
          {detail.checks.total > 0 && (
            <TabButton active={tab === "checks"} onClick={() => showTab("checks")}>
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
                pullRequest={detail.url}
                conversation={conversation}
                loading={conversationLoading}
                actions={actions}
                onComment={comment}
              />
            )}
            {tab === "overview" && (
              <PullsOverview path={path} detail={detail} onRefresh={onRefresh} />
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
          onConfirm={() => void merge(edit.method, edit.subject, edit.body, bypass)}
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
