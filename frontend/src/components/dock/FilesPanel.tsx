import { ChevronLeft } from "lucide-react"
import { EditorView } from "@codemirror/view"
import { useEffect, useMemo, useRef, useState, type KeyboardEvent } from "react"
import { createPortal } from "react-dom"
import { Notice } from "@/components/common/Notice"
import { SearchInput } from "@/components/common/SearchInput"
import { CommentBatch } from "@/components/diff/CommentBatch"
import { ImageView, PdfView } from "@/components/diff/BinaryPreview"
import { InjectMenu } from "@/components/diff/InjectMenu"
import { type Composer, ReviewSlot } from "@/components/diff/ReviewSlots"
import { FileTree } from "@/components/FileTree"
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group"
import { NO_SLOTS, threadSlots, type SlotElements } from "@/lib/codemirror-threads"
import { formatLineRef, parseDiff, type DiffFile } from "@/lib/git/diff"
import { buildTree, treeFootnote, type TreeNode } from "@/lib/git/file-tree"
import { previewKind } from "@/lib/git/preview"
import {
  updateFileBrowse,
  useFileBrowse,
  useFileSearchRequest,
  type FileBrowse,
} from "@/lib/file-browse"
import { COMPOSER_KEY } from "@/lib/pulls/review-slots"
import { matchesQuery } from "@/lib/session/command-palette"
import { addReviewComment } from "@/lib/review-comments"
import { queuePaste } from "@/lib/terminal/paste-queue"
import { useProjects } from "@/providers/projects"
import { ProjectService, System } from "@/lib/rpc"
import { useActiveSession } from "@/lib/session/use-active-session"
import { GIT_POLL, useGitStatus } from "@/lib/git/use-git-status"
import { useInject } from "@/lib/use-inject"
import { useRemoteResource } from "@/lib/use-remote-resource"
import { HIT_SELECTOR, SearchResults } from "./SearchResults"
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
  // A plain folder answers no git status, so the key below would never move and
  // the tree would only ever re-read on a remount. This tick is what replaces
  // the status for it, on the cadence an idle repository's poll settles at.
  const tick = usePlainFolderTick(path !== "" && status === null)
  // Same invalidation as the diff panel: the git-status poll doubles as the
  // signal, so a new or removed file shows up without a watcher. It rides the
  // key rather than an effect, because the key is what stands for the request.
  const key = path
    ? `${path} ${status?.files ?? 0} ${status?.added ?? 0} ${status?.deleted ?? 0} ${tick}`
    : ""
  const { data, loading, error } = useRemoteResource(
    key,
    async () => {
      // The diff feeds each row its +/- badge; a diff failure (nothing to diff)
      // just means no badges, never a broken tree — hence the swallowed catch.
      const [listing, diffText] = await Promise.all([
        ProjectService.Tree(path),
        ProjectService.DiffText(path).catch(() => ""),
      ])
      return {
        files: listing.files ?? [],
        cut: listing.cut,
        hidden: listing.hidden ?? [],
        stats: diffStatsByPath(parseDiff(diffText)),
      }
    },
    // resetOn is the path and not the key: a moved file count refreshes the
    // tree in place, while a worktree switch with no answer on file must drop
    // the previous checkout's files rather than list them under this one.
    { empty: NO_TREE, resetOn: path, cache: `dock-tree ${path}` },
  )

  // The filter reads the whole repo-relative path, so a directory name narrows
  // the tree to what is under it — one field for both halves of "find a file".
  const nameQuery = browse.mode === "name" ? browse.query : ""
  const tree = useMemo(
    () => buildTree(data.files.filter((file) => matchesQuery(file, nameQuery))),
    [data.files, nameQuery],
  )

  const rootRef = useRef<HTMLDivElement>(null)
  useFileSearchRequest(() => {
    updateFileBrowse(path, { mode: "text", open: "" })
    const box = rootRef.current?.querySelector("input")
    box?.focus()
    box?.select()
  })

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
    <div ref={rootRef} className="flex h-full flex-col">
      {/* The preview covers the tree instead of replacing it: the scroll that
          reached a file is the tree's own, and nothing outside restores it, so
          unmounting the tree would throw it away on every Back. */}
      <div className="relative flex flex-1 flex-col overflow-hidden">
        <div className="flex h-full flex-col overflow-hidden">
          <BrowseBox
            path={path}
            browse={browse}
            onFocusList={() => {
              const first = rootRef.current?.querySelector<HTMLElement>(HIT_SELECTOR)
              first?.focus()
              return first !== undefined && first !== null
            }}
          />
          {browse.mode === "text" ? (
            <SearchResults
              path={path}
              query={browse.query}
              refreshKey={key}
              listingNote={treeFootnote(data.cut, data.hidden)}
              onOpen={(rel, line) => updateFileBrowse(path, { open: rel, selected: rel, line })}
            />
          ) : (
            <TreeBody
              tree={tree}
              query={nameQuery}
              active={browse.selected}
              toggled={browse.toggled}
              onToggled={(toggled) => updateFileBrowse(path, { toggled })}
              stats={data.stats}
              cut={data.cut}
              hidden={data.hidden}
              loading={loading}
              failed={error !== null}
              onOpen={(rel) => updateFileBrowse(path, { open: rel, selected: rel, line: 0 })}
              onEditor={openInEditor}
            />
          )}
        </div>
        {browse.open !== "" && (
          <div className="absolute inset-0 z-10 bg-sidebar">
            <FilePreview
              path={path}
              rel={browse.open}
              line={browse.line}
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

interface BrowseBoxProps {
  path: string
  browse: FileBrowse
  /** Move focus to the first search hit; false when there is none. */
  onFocusList: () => boolean
}

// BrowseBox is the one field over the Code tab and the switch that says how it
// reads: a filter over the tree's names, or a search through the files' text.
function BrowseBox({ path, browse, onFocusList }: BrowseBoxProps) {
  const text = browse.mode === "text"
  const onKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === "Escape" && browse.query !== "") {
      event.preventDefault()
      updateFileBrowse(path, { query: "" })
      return
    }
    if (text && event.key === "ArrowDown" && onFocusList()) {
      event.preventDefault()
    }
  }
  return (
    <div className="flex shrink-0 items-center gap-1.5 border-b border-border p-1.5">
      <div className="min-w-0 flex-1">
        <SearchInput
          value={browse.query}
          onChange={(event) => updateFileBrowse(path, { query: event.target.value })}
          onKeyDown={onKeyDown}
          placeholder={text ? "Search in files" : "Filter by name"}
          aria-label={text ? "Search in files" : "Filter files by name"}
          className="h-7 text-xs"
        />
      </div>
      <ToggleGroup
        value={[browse.mode]}
        onValueChange={(next) =>
          next[0] && updateFileBrowse(path, { mode: next[0] as FileBrowse["mode"] })
        }
        spacing={1}
        aria-label="What the field matches"
        className="shrink-0 border border-border p-[0.1875rem]"
      >
        <ToggleGroupItem value="name" size="sm" className="h-5 px-2 text-xs">
          Name
        </ToggleGroupItem>
        <ToggleGroupItem value="text" size="sm" className="h-5 px-2 text-xs">
          Text
        </ToggleGroupItem>
      </ToggleGroup>
    </div>
  )
}

// What the tree holds before its first answer, and after a failed lookup. A
// module-level constant because useRemoteResource compares it by identity.
const NO_TREE: {
  files: string[]
  cut: boolean
  hidden: string[]
  stats: Map<string, DiffFile>
} = {
  files: [],
  cut: false,
  hidden: [],
  stats: new Map(),
}

// usePlainFolderTick advances a counter on the git poller's idle cadence while
// `on`, so a caller keyed off it re-reads on the same rhythm a repository's
// tree does. Only the plain-folder case turns it on: there is no git status
// there to notice a file the agent just wrote, and a walk is cheap next to the
// several git subprocesses the poller runs on the same path anyway. Exported
// for tests.
export function usePlainFolderTick(on: boolean): number {
  const [tick, setTick] = useState(0)
  useEffect(() => {
    if (!on) {
      return
    }
    const id = setInterval(() => setTick((n) => n + 1), GIT_POLL.slowMs)
    return () => clearInterval(id)
  }, [on])
  return tick
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
  /** The listing stopped at the walk's cap, so the tree is only part of the
   * folder. Only a plain folder can set it: git's listing is never cut. */
  cut: boolean
  /** Directory names the walk stepped over, for the same reason. Empty in a
   * repository, where .gitignore filters and lich names nothing. */
  hidden: string[]
  /** A read with nothing on screen yet. False through a refetch that has last
   * time's tree to stand on, which is what keeps a poll tick from flashing. */
  loading: boolean
  failed: boolean
  onOpen: (rel: string) => void
  onEditor: (rel: string) => void
}

// Exported for tests: the gate cannot stand FilesPanel up whole (it hangs off
// the projects provider and a live session), and the tree's states are what the
// panel is read on.
export function TreeBody({
  tree,
  query,
  active,
  toggled,
  onToggled,
  stats,
  cut,
  hidden,
  loading,
  failed,
  onOpen,
  onEditor,
}: TreeBodyProps) {
  const filtering = query.trim() !== ""
  const footnote = treeFootnote(cut, hidden)
  if (failed) {
    return <Notice>Could not read this folder</Notice>
  }
  if (loading) {
    return <Notice>Loading…</Notice>
  }
  return (
    <>
      {tree.length === 0 ? (
        <Notice>{filtering ? "No file matches" : "No files here"}</Notice>
      ) : (
        <FileTree
          tree={tree}
          active={active}
          // Suspended, not replaced: clearing the filter gives back the folders
          // the browse had open rather than a tree collapsed to the root.
          expandAll={filtering}
          toggled={toggled}
          onToggled={onToggled}
          stats={stats}
          onEditor={onEditor}
          className="min-h-0 flex-1"
          onSelect={onOpen}
        />
      )}
      {/* A listing that left something out has to say so, under the rows rather
          than over them: it is a footnote to the tree, not a state of the panel.
          It outlives the empty branch on purpose: "No file matches" is a lie
          when the listing the filter ran over was only part of the folder. */}
      {footnote !== "" && (
        <Notice className="shrink-0 border-t border-border py-2">{footnote}</Notice>
      )}
    </>
  )
}

interface FilePreviewProps {
  path: string
  rel: string
  /** The line to land on, or 0 for the top of the file. */
  line: number
  onBack: () => void
  onInject: (text: string) => void
  /** Hold a comment on these file lines for the checkout's batch. */
  onComment: (lines: string, text: string) => void
}

function FilePreview({ path, rel, line, onBack, onInject, onComment }: FilePreviewProps) {
  const kind = previewKind(rel)
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
        <span className="ml-auto shrink-0 text-2xs uppercase tracking-wide text-muted-foreground">
          read-only
        </span>
      </div>
      {kind === "pdf" ? (
        // The viewer scrolls its own pages, so it takes the panel's height
        // rather than sitting in a scroller.
        <PdfView path={path} rel={rel} gitRef="" version="" className="min-h-0 flex-1" />
      ) : (
        <div className="flex-1 overflow-y-auto">
          {kind === "image" ? (
            <div className="flex flex-col gap-2 p-3">
              <ImageView path={path} rel={rel} gitRef="" version="" />
            </div>
          ) : (
            <TextPreview
              path={path}
              rel={rel}
              line={line}
              onInject={onInject}
              onComment={onComment}
            />
          )}
        </div>
      )}
    </div>
  )
}

function TextPreview({ path, rel, line, onInject, onComment }: Omit<FilePreviewProps, "onBack">) {
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
  if (error !== null) {
    return <Notice>{error}</Notice>
  }
  if (loading) {
    return <Notice>Loading…</Notice>
  }
  return <PreviewBody text={text} rel={rel} line={line} onInject={onInject} onComment={onComment} />
}

interface PreviewBodyProps {
  text: string
  rel: string
  line: number
  onInject: (text: string) => void
  onComment: (lines: string, text: string) => void
}

// PreviewBody renders the file in a read-only CodeMirror view whose selection
// drives the same inject context menu as the diff review — file lines map
// straight through (doc line === file line), so the range needs no remap, and
// the composer hangs under the last line the selection covered.
//
// Only the comment for the session is offered. The other one is a review
// comment on a pull request, which GitHub anchors to a line of *its* diff: a
// line of the whole file is not that, even when the file is one the PR touched.
function PreviewBody({ text, rel, line, onInject, onComment }: PreviewBodyProps) {
  const [elements, setElements] = useState<SlotElements>(NO_SLOTS)
  // Memoised because it is part of the view's identity: a new one would rebuild
  // the editor on every render.
  const slots = useMemo(() => threadSlots(setElements), [])
  const { containerRef, getSelectedLines, view } = useFileEditor(text, rel, slots.extension)
  const [selection, setSelection] = useState<Composer | null>(null)
  const [composer, setComposer] = useState<Composer | null>(null)

  // A search hit opens on its line, selected: the selection is what the inject
  // menu and the comment read, so the line found is one right-click from both.
  // Focused too, because the viewer draws the native selection, which an
  // unfocused editor does not show.
  useEffect(() => {
    if (!view || line < 1 || line > view.state.doc.lines) {
      return
    }
    const target = view.state.doc.line(line)
    view.dispatch({
      selection: { anchor: target.from, head: target.to },
      effects: EditorView.scrollIntoView(target.from, { y: "center" }),
    })
    view.focus()
  }, [view, line])

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
