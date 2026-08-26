import { ChevronLeft } from "lucide-react"
import { useEffect, useMemo, useState } from "react"
import { createPortal } from "react-dom"
import { Notice } from "@/components/common/Notice"
import { SearchInput } from "@/components/common/SearchInput"
import { CommentBatch } from "@/components/diff/CommentBatch"
import { InjectMenu } from "@/components/diff/InjectMenu"
import { type Composer, ReviewSlot } from "@/components/diff/ReviewSlots"
import { FileTree } from "@/components/FileTree"
import { threadSlots, type SlotElements } from "@/lib/codemirror-threads"
import { formatLineRef, parseDiff, type DiffFile } from "@/lib/git/diff"
import { buildTree, type TreeNode } from "@/lib/git/file-tree"
import { updateFileBrowse, useFileBrowse } from "@/lib/file-browse"
import { COMPOSER_KEY } from "@/lib/pulls/review-slots"
import { matchesQuery } from "@/lib/session/command-palette"
import { addReviewComment } from "@/lib/review-comments"
import { queuePaste } from "@/lib/terminal/paste-queue"
import { useProjects } from "@/providers/projects"
import { ProjectService, System } from "@/lib/rpc"
import { useActiveSession } from "@/lib/session/use-active-session"
import { useGitStatus } from "@/lib/git/use-git-status"
import { useInject } from "@/lib/use-inject"
import { useRemoteResource } from "@/lib/use-remote-resource"
import { useFileEditor } from "./useFileEditor"

// FilesPanel is the Files tab of the right dock: a read-only tree of the active
// session's files — tracked and untracked in a repository, whatever is on disk
// in a plain folder — master-detail with an in-dock preview. It follows the
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
  // Everything the browse is — filter, folds, marked row, preview — lives in a
  // store keyed by this checkout, because the dock unmounts this panel on every
  // flip to the Review tab (see file-browse). Keying by path is also what keeps
  // one worktree's browse off another's tree, which the panel used to do by
  // clearing on a path change.
  const browse = useFileBrowse(path)
  // Same invalidation as the diff panel: the git-status poll doubles as the
  // signal, so a new or removed file shows up without a watcher. It rides the
  // key rather than an effect, because the key is what stands for the request.
  const key = path
    ? `${path} ${status?.files ?? 0} ${status?.added ?? 0} ${status?.deleted ?? 0}`
    : ""
  const { data, loading, error } = useRemoteResource(
    key,
    async () => {
      // The diff feeds each row its +/- badge; a diff failure (nothing to diff)
      // just means no badges, never a broken tree — hence the swallowed catch.
      const [files, diffText] = await Promise.all([
        ProjectService.Tree(path),
        ProjectService.DiffText(path).catch(() => ""),
      ])
      return { files: files ?? [], stats: diffStatsByPath(parseDiff(diffText)) }
    },
    // resetOn is the path and not the key: a moved file count refreshes the
    // tree in place, while a worktree switch with no answer on file must drop
    // the previous checkout's files rather than list them under this one.
    { empty: NO_TREE, resetOn: path, cache: `dock-tree ${path}` },
  )

  // The filter reads the whole repo-relative path, so a directory name narrows
  // the tree to what is under it — one field for both halves of "find a file".
  const tree = useMemo(
    () => buildTree(data.files.filter((file) => matchesQuery(file, browse.query))),
    [data.files, browse.query],
  )

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
      {/* The preview covers the tree instead of replacing it: the scroll that
          reached a file is the tree's own, and nothing outside restores it, so
          unmounting the tree would throw it away on every Back. */}
      <div className="relative flex flex-1 flex-col overflow-hidden">
        <div className="flex h-full flex-col overflow-hidden">
          <div className="shrink-0 border-b border-border p-1.5">
            <SearchInput
              value={browse.query}
              onChange={(event) => updateFileBrowse(path, { query: event.target.value })}
              placeholder="Filter by name"
              aria-label="Filter files by name"
              spellCheck={false}
              className="h-7 text-xs"
            />
          </div>
          <TreeBody
            tree={tree}
            query={browse.query}
            active={browse.selected}
            toggled={browse.toggled}
            onToggled={(toggled) => updateFileBrowse(path, { toggled })}
            stats={data.stats}
            loading={loading}
            failed={error !== null}
            onOpen={(rel) => updateFileBrowse(path, { open: rel, selected: rel })}
            onEditor={openInEditor}
          />
        </div>
        {browse.open !== "" && (
          <div className="absolute inset-0 z-10 bg-sidebar">
            <FilePreview
              path={path}
              rel={browse.open}
              onBack={() => updateFileBrowse(path, { open: "" })}
              onInject={inject}
              onComment={(lines, text) => addReviewComment(path, browse.open, lines, text)}
            />
          </div>
        )}
      </div>
      {/* Below the tree as well as the preview: a batch written file by file
          must not vanish on the way back to pick the next one. */}
      <CommentBatch target={path} onInject={inject} />
    </div>
  )
}

// What the tree holds before its first answer, and after a failed lookup. A
// module-level constant because useRemoteResource compares it by identity.
const NO_TREE: { files: string[]; stats: Map<string, DiffFile> } = {
  files: [],
  stats: new Map(),
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
  tree: TreeNode[]
  /** The filter the tree was narrowed by; empty means the whole checkout. */
  query: string
  /** The file last opened, so Back lands on a row that reads as current. */
  active: string
  toggled: ReadonlySet<string>
  onToggled: (toggled: ReadonlySet<string>) => void
  stats: Map<string, DiffFile>
  /** A read with nothing on screen yet. False through a refetch that has last
   * time's tree to stand on, which is what keeps a poll tick from flashing. */
  loading: boolean
  failed: boolean
  onOpen: (rel: string) => void
  onEditor: (rel: string) => void
}

function TreeBody({
  tree,
  query,
  active,
  toggled,
  onToggled,
  stats,
  loading,
  failed,
  onOpen,
  onEditor,
}: TreeBodyProps) {
  const filtering = query.trim() !== ""
  if (failed) {
    return <Notice>Could not read this folder</Notice>
  }
  if (loading) {
    return <Notice>Loading…</Notice>
  }
  if (tree.length === 0) {
    return <Notice>{filtering ? "No file matches" : "No files here"}</Notice>
  }
  return (
    <FileTree
      tree={tree}
      active={active}
      // Suspended, not replaced: clearing the filter gives back the folders the
      // browse had open rather than a tree collapsed to the root.
      expandAll={filtering}
      toggled={toggled}
      onToggled={onToggled}
      stats={stats}
      onEditor={onEditor}
      className="h-full"
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
  // Filed like the tree above it: a preview left open is restored on the way
  // back from the Review tab, and painting it from a skeleton every time would
  // undo half of what restoring it was for. resetOn keeps one file's text from
  // appearing for a moment under another file's name.
  const {
    data: text,
    loading,
    error,
  } = useRemoteResource(`${path} ${rel}`, () => ProjectService.ReadFile(path, rel), {
    empty: "",
    resetOn: rel,
    cache: `dock-file ${path} ${rel}`,
  })

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
        ) : loading ? (
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
