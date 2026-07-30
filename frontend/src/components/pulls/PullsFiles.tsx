import { PanelLeftClose, PanelLeftOpen } from "lucide-react"
import { useMemo, useRef, useState, useSyncExternalStore } from "react"
import { IconAction } from "@/components/common/IconAction"
import { Notice } from "@/components/common/Notice"
import { CommentBatch } from "@/components/diff/CommentBatch"
import { CollapseAllAction, useDiffBulk } from "@/components/diff/diff-bulk"
import { FileDiff } from "@/components/diff/FileDiff"
import { DiffStat } from "@/components/DiffStat"
import { FileTree } from "@/components/FileTree"
import { SkeletonLines } from "@/components/common/SkeletonLines"
import { Skeleton } from "@/components/ui/skeleton"
import { buildTree } from "@/lib/git/file-tree"
import type { DraftReviewComment, ReviewThread as Thread } from "@/lib/api-types"
import type { DiffReview } from "@/components/diff/ReviewSlots"
import type { ThreadActions } from "./ReviewThread"
import {
  addDraftComment,
  draftsOnFile,
  editDraftComment,
  pendingReview,
  removeDraftComment,
  subscribePendingReview,
} from "@/lib/pulls/pending-review-store"
import {
  fileFingerprint,
  isViewed,
  setViewed,
  subscribeViewed,
  viewedFiles,
} from "@/lib/pulls/pull-request-viewed"
import { usePullRequestDiff } from "@/lib/pulls/use-pull-request-diff"
import { addReviewComment } from "@/lib/review-comments"
import { usePanelVisible } from "@/lib/use-panel-visible"

// The file tree is a navigator, not the review itself: hiding it hands the
// whole width to the diff. Remembered in localStorage like every other UI pref,
// so the choice survives leaving the screen.
const TREE_HIDDEN_KEY = "lich.pulls.tree.hidden"

// Ragged widths, so the placeholders read as file names and code rather than a
// stack of identical bars.
const TREE_ROWS = ["w-28", "w-40", "w-24", "w-36", "w-32", "w-44", "w-24", "w-36"]
const CODE_ROWS = ["w-3/5", "w-4/5", "w-2/5", "w-11/12", "w-1/2", "w-3/4", "w-1/3", "w-2/3"]

// The diff is a gh round-trip, so this tab is empty for about a second. A bare
// "Loading…" line reads as a broken tab on a screen that wide; the skeleton
// stands in for the shape that arrives — tree, toolbar, then file cards.
function FilesSkeleton({ tree }: { tree: boolean }) {
  return (
    <div className="flex h-full" aria-busy>
      {tree && (
        <div className="flex w-60 shrink-0 flex-col gap-3 border-r border-border p-3">
          <SkeletonLines widths={TREE_ROWS} />
        </div>
      )}
      <div className="flex flex-1 flex-col overflow-hidden">
        <div className="flex flex-none items-center gap-2 border-b border-border px-3 py-2">
          <Skeleton className="size-3.5" />
          <Skeleton className="ml-auto h-3 w-20" />
        </div>
        <div className="flex flex-col gap-7 p-3">
          {[0, 1, 2].map((card) => (
            <div key={card} className="flex flex-col gap-3">
              <div className="flex items-center gap-2">
                <Skeleton className="size-3.5" />
                <Skeleton className="size-5" />
                <Skeleton className="h-3 w-40" />
                <Skeleton className="h-3 w-24" />
                <Skeleton className="ml-auto h-3 w-14" />
              </div>
              <div className="flex max-w-3xl flex-col gap-2 pl-9">
                <SkeletonLines widths={CODE_ROWS} height="h-2.5" />
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

interface PullsFilesProps {
  path: string
  /** Which pull request's diff to fetch. */
  number: number
  /** The checkout's HEAD; a new commit refetches the diff. */
  head: string
  /** Identity of the pull request being reviewed (its URL) — what the Viewed
   * ticks and the draft review are keyed by, so two PRs never share them. */
  pullRequest: string
  /** Write into the session's terminal; false when no session took it. */
  onInject: (text: string) => boolean
  /** Every thread of the pull request; each file takes the ones anchored in it. */
  threads: Thread[] | null
  actions: ThreadActions
}

// PullsFiles is the "Files changed" tab of the Pulls screen: a changed-files
// tree on the left (click to jump) beside the PR's diff, rendered with the same
// FileDiff cards as the Review dock — read-only, no discard. Inject still works,
// so a PR file can be referenced into the session's terminal, and a comment on
// the selection joins the batch this tab sends as one prompt. Each file can be
// ticked off as viewed, which folds it away and counts toward the header total.
export function PullsFiles({
  path,
  number,
  head,
  pullRequest,
  onInject,
  threads,
  actions,
}: PullsFilesProps) {
  const { files, error } = usePullRequestDiff(path, head, number)
  const rows = useRef<Map<string, HTMLElement>>(new Map())
  const [active, setActive] = useState<string | null>(null)
  const review = useSyncExternalStore(subscribePendingReview, () => pendingReview(pullRequest))
  // Every file mounts its own CodeMirror, so a wide PR earns a way to fold them
  // all at once — same directive the Review dock hands its panel.
  const [bulk, toggleAll] = useDiffBulk()
  const [treeOpen, toggleTree] = usePanelVisible(TREE_HIDDEN_KEY)
  const viewed = useSyncExternalStore(subscribeViewed, () => viewedFiles(pullRequest))
  // A tick is against the file's content, so a new commit unticks exactly the
  // files it rewrote. Recomputed only when the diff itself changes.
  const fingerprints = useMemo(
    () => new Map((files ?? []).map((file) => [file.newPath, fileFingerprint(file)])),
    [files],
  )
  // Structure only; the per-file +/- lives on each diff's header, the way
  // GitHub shows it.
  const tree = useMemo(() => buildTree((files ?? []).map((file) => file.newPath)), [files])

  if (error) {
    return <Notice className="px-4 py-6 text-sm">Couldn’t load the diff: {error}</Notice>
  }
  if (files === null) {
    return <FilesSkeleton tree={treeOpen} />
  }
  if (files.length === 0) {
    return <Notice className="px-4 py-6 text-sm">No file changes</Notice>
  }

  const added = files.reduce((sum, file) => sum + file.added, 0)
  const deleted = files.reduce((sum, file) => sum + file.deleted, 0)
  const viewedCount = files.filter((file) =>
    isViewed(viewed, file.newPath, fingerprints.get(file.newPath) ?? ""),
  ).length

  const jumpTo = (target: string) => {
    setActive(target)
    rows.current.get(target)?.scrollIntoView({ block: "start", behavior: "smooth" })
  }

  // What one file's diff gets laid over it: its own threads, its own drafts, and
  // the three writes that change them. The draft store is keyed by pull request,
  // so the file only has to say which comments are its.
  const reviewFor = (filePath: string): DiffReview => ({
    threads: (threads ?? []).filter((thread) => thread.path === filePath),
    drafts: [...draftsOnFile(review, filePath, "RIGHT"), ...draftsOnFile(review, filePath, "LEFT")],
    actions,
    onAdd: (comment: DraftReviewComment) => addDraftComment(pullRequest, comment),
    onEdit: (index: number, body: string) => editDraftComment(pullRequest, index, body),
    onRemove: (index: number) => removeDraftComment(pullRequest, index),
  })

  return (
    <div className="flex h-full">
      {treeOpen && (
        <div className="w-60 shrink-0 overflow-y-auto border-r border-border">
          <FileTree tree={tree} active={active} defaultOpen onSelect={jumpTo} />
        </div>
      )}
      <div className="flex flex-1 flex-col overflow-hidden">
        {/* Outside the scroll area, not sticky: the totals and the panel toggle
            stay put without fighting each file's own sticky header. */}
        <div className="flex flex-none items-center justify-end gap-2 border-b border-border px-3 py-2 text-xs text-muted-foreground">
          <span className="mr-auto flex items-center gap-2">
            <IconAction
              label={treeOpen ? "Hide the file tree" : "Show the file tree"}
              onClick={toggleTree}
            >
              {treeOpen ? (
                <PanelLeftClose className="size-3.5" />
              ) : (
                <PanelLeftOpen className="size-3.5" />
              )}
            </IconAction>
            {viewedCount > 0 && (
              <span className="tabular-nums">
                {viewedCount} of {files.length} viewed
              </span>
            )}
          </span>
          <span className="flex items-center gap-1.5">
            <DiffStat added={added} deleted={deleted} />
          </span>
          <CollapseAllAction open={bulk.open} onToggle={toggleAll} />
        </div>
        <div className="flex-1 overflow-y-auto">
          <div className="flex flex-col p-3 [&>div:not(:first-child)]:mt-2.5 [&>div:not(:first-child)]:border-t [&>div:not(:first-child)]:border-border [&>div:not(:first-child)]:pt-2.5">
            {files.map((file) => (
              <div
                key={file.newPath}
                ref={(el) => {
                  if (el) {
                    rows.current.set(file.newPath, el)
                  } else {
                    rows.current.delete(file.newPath)
                  }
                }}
              >
                <FileDiff
                  file={file}
                  onInject={onInject}
                  onSessionComment={(path, lines, text) =>
                    addReviewComment(pullRequest, path, lines, text)
                  }
                  bulk={bulk}
                  viewed={isViewed(viewed, file.newPath, fingerprints.get(file.newPath) ?? "")}
                  onViewed={(next) =>
                    setViewed(pullRequest, file.newPath, fingerprints.get(file.newPath) ?? "", next)
                  }
                  review={reviewFor(file.newPath)}
                />
              </div>
            ))}
          </div>
        </div>
        <CommentBatch target={pullRequest} onInject={onInject} />
      </div>
    </div>
  )
}
