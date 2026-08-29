import { PanelLeftClose, PanelLeftOpen } from "lucide-react"
import { useCallback, useMemo, useRef, useSyncExternalStore } from "react"
import { IconAction } from "@/components/common/IconAction"
import { ResizeHandle } from "@/components/common/ResizeHandle"
import { Notice } from "@/components/common/Notice"
import { CommentBatch } from "@/components/diff/CommentBatch"
import { CollapseAllAction, useDiffBulk } from "@/components/diff/diff-bulk"
import { FileDiff } from "@/components/diff/FileDiff"
import { DiffStat } from "@/components/DiffStat"
import { FileTree } from "@/components/FileTree"
import { SkeletonLines } from "@/components/common/SkeletonLines"
import { Skeleton } from "@/components/ui/skeleton"
import { buildTree } from "@/lib/git/file-tree"
import type { ReviewThread as Thread } from "@/lib/api-types"
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
import { useActiveFile } from "@/lib/pulls/use-active-file"
import { usePullRequestDiff } from "@/lib/pulls/use-pull-request-diff"
import { addReviewComment } from "@/lib/review-comments"
import { ProjectService } from "@/lib/rpc"
import { usePanelVisible } from "@/lib/use-panel-visible"
import { usePanelWidth } from "@/lib/use-panel-width"

// The file tree is a navigator, not the review itself: hiding it hands the
// whole width to the diff. Remembered in localStorage like every other UI pref,
// so the choice survives leaving the screen.
const TREE_HIDDEN_KEY = "lich.pulls.tree.hidden"

// A monorepo's paths are long enough that no fixed width fits them, so the tree
// drags like every other side panel — narrow when the diff matters more, wide
// when the navigating does.
const WIDTH_KEY = "lich.pulls.tree.width"
const MIN_REM = 12
const MAX_REM = 40
const DEFAULT_REM = 15

// Ragged widths, so the placeholders read as file names and code rather than a
// stack of identical bars.
const TREE_ROWS = ["w-28", "w-40", "w-24", "w-36", "w-32", "w-44", "w-24", "w-36"]
const CODE_ROWS = ["w-3/5", "w-4/5", "w-2/5", "w-11/12", "w-1/2", "w-3/4", "w-1/3", "w-2/3"]

// The diff is a gh round-trip, so this tab is empty for about a second. A bare
// "Loading…" line reads as a broken tab on a screen that wide; the skeleton
// stands in for the shape that arrives — tree, toolbar, then file cards.
function FilesSkeleton({ tree, width }: { tree: boolean; width: number }) {
  return (
    <div className="flex h-full" aria-busy>
      {tree && (
        <div
          className="flex shrink-0 flex-col gap-3 border-r border-border p-3"
          style={{ width: `${width}rem` }}
        >
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
  /** The checkout the review runs from: where the diff is fetched, and what the
   * session comments are held under — the dock browsing the same checkout adds
   * to that batch too. */
  path: string
  /** Which pull request's diff to fetch. */
  number: number
  /** The commit the diff's new side stands at — the PR's head, which the
   * expander reads unchanged lines from. "" while the detail has no commit to
   * name, and then the diff shows what git printed and nothing more. */
  headOid: string
  /** The checkout's HEAD; a new commit refetches the diff. */
  head: string
  /** Identity of the pull request being reviewed (its URL) — what the Viewed
   * ticks and the draft review are keyed by, so two PRs never share them. The
   * session comments are not among them: they are keyed by the checkout. */
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
  headOid,
  head,
  pullRequest,
  onInject,
  threads,
  actions,
}: PullsFilesProps) {
  const { files, error } = usePullRequestDiff(path, head, number)
  // Read against the PR's head and never against the checkout: the branch under
  // review is usually not the one on disk, and often not in this clone at all
  // (project.FileLines then asks GitHub for it).
  const expand = useCallback(
    (rel: string, from: number, to: number) =>
      ProjectService.FileLines(path, rel, headOid, from, to),
    [path, headOid],
  )
  const rows = useRef<Map<string, HTMLElement>>(new Map())
  const [active, selectFile] = useActiveFile(pullRequest)
  const review = useSyncExternalStore(subscribePendingReview, () => pendingReview(pullRequest))
  // Every file mounts its own CodeMirror, so a wide PR earns a way to fold them
  // all at once — same directive the Review dock hands its panel.
  const [bulk, toggleAll] = useDiffBulk()
  const [treeOpen, toggleTree] = usePanelVisible(TREE_HIDDEN_KEY)
  const { width, handleProps } = usePanelWidth({
    storageKey: WIDTH_KEY,
    minRem: MIN_REM,
    maxRem: MAX_REM,
    defaultRem: DEFAULT_REM,
    edge: "right",
  })
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
  // What one file's diff gets laid over it: its own threads, its own drafts, and
  // the three writes that change them. The draft store is keyed by pull request,
  // so the file only has to say which comments are its.
  //
  // Memoised because each of these rides the identity of its file's CodeMirror
  // decorations: a fresh object per render would re-dispatch every mounted
  // editor's gaps on every render of this screen, and this screen re-renders
  // for things the diff does not care about — a tick, a tree click, a keystroke
  // in the review summary. Hence the drafts and not the whole review: only the
  // line comments are laid over the diff.
  const drafts = review.comments
  const reviews = useMemo(() => {
    const byPath = new Map<string, DiffReview>()
    for (const file of files ?? []) {
      byPath.set(file.newPath, {
        threads: (threads ?? []).filter((thread) => thread.path === file.newPath),
        drafts: [
          ...draftsOnFile(drafts, file.newPath, "RIGHT"),
          ...draftsOnFile(drafts, file.newPath, "LEFT"),
        ],
        actions,
        onAdd: (comment) => addDraftComment(pullRequest, comment),
        onEdit: (index, body) => editDraftComment(pullRequest, index, body),
        onRemove: (index) => removeDraftComment(pullRequest, index),
      })
    }
    return byPath
  }, [files, threads, drafts, actions, pullRequest])

  if (error) {
    return <Notice className="px-4 py-6 text-sm">Couldn’t load the diff: {error}</Notice>
  }
  if (files === null) {
    return <FilesSkeleton tree={treeOpen} width={width} />
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
    selectFile(target)
    rows.current.get(target)?.scrollIntoView({ block: "start", behavior: "smooth" })
  }

  return (
    <div className="flex h-full">
      {treeOpen && (
        <div className="relative shrink-0 border-r border-border" style={{ width: `${width}rem` }}>
          <FileTree tree={tree} active={active} defaultOpen className="h-full" onSelect={jumpTo} />
          <ResizeHandle edge="right" label="Resize the file tree" handleProps={handleProps} />
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
                  onSessionComment={(file, lines, text) =>
                    addReviewComment(path, file, lines, text)
                  }
                  bulk={bulk}
                  viewed={isViewed(viewed, file.newPath, fingerprints.get(file.newPath) ?? "")}
                  onViewed={(next) =>
                    setViewed(pullRequest, file.newPath, fingerprints.get(file.newPath) ?? "", next)
                  }
                  review={reviews.get(file.newPath)}
                  onExpand={headOid ? expand : undefined}
                />
              </div>
            ))}
          </div>
        </div>
        {/* Keyed by the checkout, not by the pull request: these notes are
            going to that checkout's session, and the dock's own comments are
            waiting in the same batch. */}
        <CommentBatch target={path} onInject={onInject} />
      </div>
    </div>
  )
}
