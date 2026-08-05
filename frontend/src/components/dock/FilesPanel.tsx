import { ChevronLeft } from "lucide-react"
import { useCallback, useEffect, useMemo, useState } from "react"
import { createPortal } from "react-dom"
import { Notice } from "@/components/common/Notice"
import { CommentBatch } from "@/components/diff/CommentBatch"
import { InjectMenu } from "@/components/diff/InjectMenu"
import { type Composer, ReviewSlot } from "@/components/diff/ReviewSlots"
import { FileTree } from "@/components/FileTree"
import { threadSlots, type SlotElements } from "@/lib/codemirror-threads"
import { formatLineRef, parseDiff, type DiffFile } from "@/lib/git/diff"
import { buildTree, type TreeNode } from "@/lib/git/file-tree"
import { COMPOSER_KEY } from "@/lib/pulls/review-slots"
import { addReviewComment } from "@/lib/review-comments"
import { queuePaste } from "@/lib/terminal/paste-queue"
import { useProjects } from "@/providers/projects"
import { ProjectService, System } from "@/lib/rpc"
import { useActiveSession } from "@/lib/session/use-active-session"
import { useGitStatus } from "@/lib/git/use-git-status"
import { useInject } from "@/lib/use-inject"
import { errorText } from "@/lib/utils"
import { useFileEditor } from "./useFileEditor"

// FilesPanel is the Files tab of the right dock: a read-only tree of the active
// session's tracked files, master-detail with an in-dock preview. It follows the
// active session like the review panel — a worktree session browses its
// checkout, not the project root — so clicking a file opens it beside the same
// terminal it belongs to. It never edits; clicks only navigate, inject
// path/line references into the session's PTY, or leave a comment on the lines
// under the selection. That comment joins the checkout's one batch, the same
// one the review panel and a pull request's diff write into: reading a file
// whole is part of reviewing, and a note taken there belongs with the rest.
export function FilesPanel() {
  const { projectId, sessionId, path } = useActiveSession()
  const { newSession, activateSession } = useProjects()
  const inject = useInject(sessionId)
  const status = useGitStatus(path)
  const [tree, setTree] = useState<TreeNode[] | null>(null)
  const [stats, setStats] = useState<Map<string, DiffFile>>(new Map())
  const [failed, setFailed] = useState(false)
  const [open, setOpen] = useState<string | null>(null)

  const refresh = useCallback(async () => {
    if (!path) {
      return
    }
    try {
      // The diff feeds each row its +/- badge; a diff failure (nothing to diff)
      // just means no badges, never a broken tree — hence the swallowed catch.
      const [files, diffText] = await Promise.all([
        ProjectService.Tree(path),
        ProjectService.DiffText(path).catch(() => ""),
      ])
      setTree(buildTree(files ?? []))
      setStats(diffStatsByPath(parseDiff(diffText)))
      setFailed(false)
    } catch {
      setTree([])
      setStats(new Map())
      setFailed(true)
    }
  }, [path])

  // Same invalidation as the diff panel: the git-status poll doubles as the
  // signal, so a new or removed file shows up without a watcher.
  useEffect(() => {
    void refresh()
  }, [refresh, status?.files, status?.added, status?.deleted])

  // A worktree switch changes path; drop any preview from the old tree.
  useEffect(() => {
    setOpen(null)
  }, [path])

  if (!projectId) {
    return null
  }

  // Right-click → Open in editor. The backend either launched a GUI editor
  // detached (empty reply) or, for a terminal editor like vim, handed back the
  // command to run: spawn a shell session at this checkout and let the paste
  // queue deliver it once the PTY exists, the way the self-update flow does.
  const openInEditor = (rel: string) => {
    if (!path) {
      return
    }
    void System.OpenInEditor(path, rel)
      .then((command) => {
        if (!command) {
          return
        }
        const id = newSession(projectId, "shell", path)
        queuePaste(id, `${command}\n`)
        activateSession(projectId, id)
      })
      .catch(() => undefined)
  }

  return (
    <div className="flex h-full flex-col">
      <div className="flex flex-1 flex-col overflow-hidden">
        {open !== null ? (
          <FilePreview
            path={path}
            rel={open}
            onBack={() => setOpen(null)}
            onInject={inject}
            onComment={(lines, text) => addReviewComment(path, open, lines, text)}
          />
        ) : (
          <TreeBody
            tree={tree}
            stats={stats}
            failed={failed}
            onOpen={setOpen}
            onEditor={openInEditor}
          />
        )}
      </div>
      {/* Below the tree as well as the preview: a batch written file by file
          must not vanish on the way back to pick the next one. */}
      <CommentBatch target={path} onInject={inject} />
    </div>
  )
}

// diffStatsByPath keys each changed file's +/- counts by its current path so a
// tree row can look up its own line delta. parseDiff already computed the counts
// for the review panel; this only reshapes them for lookup.
function diffStatsByPath(files: DiffFile[]): Map<string, DiffFile> {
  const map = new Map<string, DiffFile>()
  for (const file of files) {
    const key = file.newPath || file.oldPath
    if (key) {
      map.set(key, file)
    }
  }
  return map
}

interface TreeBodyProps {
  tree: TreeNode[] | null
  stats: Map<string, DiffFile>
  failed: boolean
  onOpen: (rel: string) => void
  onEditor: (rel: string) => void
}

function TreeBody({ tree, stats, failed, onOpen, onEditor }: TreeBodyProps) {
  if (failed) {
    return <Notice>Not a git repository</Notice>
  }
  if (tree === null) {
    return <Notice>Loading…</Notice>
  }
  if (tree.length === 0) {
    return <Notice>No tracked files</Notice>
  }
  return (
    <FileTree
      tree={tree}
      stats={stats}
      onEditor={onEditor}
      className="h-full overflow-y-auto"
      onSelect={onOpen}
    />
  )
}

interface FilePreviewProps {
  path: string
  rel: string
  onBack: () => void
  onInject: (text: string) => void
  /** Hold a comment on these file lines for the checkout's batch. */
  onComment: (lines: string, text: string) => void
}

function FilePreview({ path, rel, onBack, onInject, onComment }: FilePreviewProps) {
  const [text, setText] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let alive = true
    setText(null)
    setError(null)
    ProjectService.ReadFile(path, rel)
      .then((content) => {
        if (alive) {
          setText(content)
        }
      })
      .catch((err: unknown) => {
        if (alive) {
          setError(errorText(err))
        }
      })
    return () => {
      alive = false
    }
  }, [path, rel])

  return (
    <div className="flex h-full flex-col">
      <div className="flex h-8 shrink-0 items-center gap-1.5 border-b border-border px-2 text-xs">
        <button
          type="button"
          onClick={onBack}
          aria-label="Back to file tree"
          className="flex size-5 shrink-0 items-center justify-center rounded text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground"
        >
          <ChevronLeft className="size-4" />
        </button>
        <span className="truncate font-mono" title={rel}>
          {rel}
        </span>
        <span className="ml-auto shrink-0 text-[0.5625rem] uppercase tracking-wide text-muted-foreground">
          read-only
        </span>
      </div>
      <div className="flex-1 overflow-y-auto">
        {error !== null ? (
          <Notice>{error}</Notice>
        ) : text === null ? (
          <Notice>Loading…</Notice>
        ) : (
          <PreviewBody text={text} rel={rel} onInject={onInject} onComment={onComment} />
        )}
      </div>
    </div>
  )
}

interface PreviewBodyProps {
  text: string
  rel: string
  onInject: (text: string) => void
  onComment: (lines: string, text: string) => void
}

const NO_SLOTS: SlotElements = new Map()

// PreviewBody renders the file in a read-only CodeMirror view whose selection
// drives the same inject context menu as the diff review — file lines map
// straight through (doc line === file line), so the range needs no remap, and
// the composer hangs under the last line the selection covered.
//
// Only the comment for the session is offered. The other one is a review
// comment on a pull request, which GitHub anchors to a line of *its* diff: a
// line of the whole file is not that, even when the file is one the PR touched.
function PreviewBody({ text, rel, onInject, onComment }: PreviewBodyProps) {
  const [elements, setElements] = useState<SlotElements>(NO_SLOTS)
  // Memoised because it is part of the view's identity: a new one would rebuild
  // the editor on every render.
  const slots = useMemo(() => threadSlots(setElements), [])
  const { containerRef, getSelectedLines, view } = useFileEditor(text, rel, slots.extension)
  const [selection, setSelection] = useState<Composer | null>(null)
  const [composer, setComposer] = useState<Composer | null>(null)

  // Keyed off the composer's line rather than the composer, so typing into it
  // does not re-dispatch the gap on every keystroke.
  const composerLine = composer?.docLine ?? 0
  useEffect(() => {
    if (view) {
      slots.update(view, composerLine > 0 ? [{ key: COMPOSER_KEY, docLine: composerLine }] : [])
    }
  }, [view, slots, composerLine])

  const file = (): void => {
    const body = composer?.body.trim() ?? ""
    if (!composer || body === "") {
      return
    }
    onComment(composer.lines, body)
    setComposer(null)
  }

  return (
    <>
      <InjectMenu
        path={rel}
        containerRef={containerRef}
        lineRef={selection?.lines ?? null}
        // Resolve the selection when the menu opens, not on every change.
        onOpenChange={(open) => {
          if (!open) {
            return
          }
          const range = getSelectedLines()
          setSelection(
            range && {
              kind: "session",
              lines: formatLineRef({ start: range.from, end: range.to }),
              range: { start: range.from, end: range.to },
              docLine: range.to,
              body: "",
            },
          )
        }}
        onInject={onInject}
        onSessionComment={() => selection && setComposer(selection)}
      />
      {[...elements].map(([key, element]) =>
        createPortal(
          <ReviewSlot
            slotKey={key}
            composer={composer}
            onComposerChange={(body) => setComposer((held) => held && { ...held, body })}
            onComposerSubmit={file}
            onComposerCancel={() => setComposer(null)}
          />,
          element,
          key,
        ),
      )}
    </>
  )
}
