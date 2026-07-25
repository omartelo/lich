import { PanelLeftClose, PanelLeftOpen } from "lucide-react"
import { useMemo, useRef, useState, useSyncExternalStore } from "react"
import { IconAction } from "@/components/common/IconAction"
import { Notice } from "@/components/common/Notice"
import { CollapseAllAction, useDiffBulk } from "@/components/diff/diff-bulk"
import { FileDiff } from "@/components/diff/FileDiff"
import { DiffStat } from "@/components/DiffStat"
import { FileTree } from "@/components/FileTree"
import { buildTree } from "@/lib/git/file-tree"
import {
  fileFingerprint,
  isViewed,
  setViewed,
  subscribeViewed,
  viewedFiles,
} from "@/lib/pulls/pull-request-viewed"
import { usePullRequestDiff } from "@/lib/pulls/use-pull-request-diff"

// The file tree is a navigator, not the review itself: hiding it hands the
// whole width to the diff. Remembered in localStorage like every other UI pref,
// so the choice survives leaving the screen.
const TREE_HIDDEN_KEY = "lich.pulls.tree.hidden"

interface PullsFilesProps {
  path: string
  /** The checkout's HEAD; a new commit refetches the diff. */
  head: string
  /** Identity of the pull request being reviewed (its URL) — what the Viewed
   * ticks are keyed by, so two PRs never share them. */
  pullRequest: string
  onInject: (text: string) => void
}

// PullsFiles is the "Files changed" tab of the Pulls screen: a changed-files
// tree on the left (click to jump) beside the PR's diff, rendered with the same
// FileDiff cards as the Review dock — read-only, no discard. Inject still works,
// so a PR file can be referenced into the session's terminal. Each file can be
// ticked off as viewed, which folds it away and counts toward the header total.
export function PullsFiles({ path, head, pullRequest, onInject }: PullsFilesProps) {
  const { files, error } = usePullRequestDiff(path, head)
  const rows = useRef<Map<string, HTMLElement>>(new Map())
  const [active, setActive] = useState<string | null>(null)
  // Every file mounts its own CodeMirror, so a wide PR earns a way to fold them
  // all at once — same directive the Review dock hands its panel.
  const [bulk, toggleAll] = useDiffBulk()
  const [treeOpen, setTreeOpen] = useState(() => localStorage.getItem(TREE_HIDDEN_KEY) !== "1")
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
    return <Notice className="px-4 py-6 text-sm">Loading…</Notice>
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

  const toggleTree = () => {
    const next = !treeOpen
    setTreeOpen(next)
    localStorage.setItem(TREE_HIDDEN_KEY, next ? "0" : "1")
  }

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
                  bulk={bulk}
                  viewed={isViewed(viewed, file.newPath, fingerprints.get(file.newPath) ?? "")}
                  onViewed={(next) =>
                    setViewed(pullRequest, file.newPath, fingerprints.get(file.newPath) ?? "", next)
                  }
                />
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  )
}
