import {useCallback, useEffect, useState} from "react"
import {ChevronDown, ChevronLeft, ChevronRight, Folder, FolderOpen} from "lucide-react"
import {ProjectService, Terminal as TerminalService} from "@/lib/rpc"
import {useActiveSession} from "@/lib/useActiveSession"
import {useGitStatus} from "@/lib/useGitStatus"
import {buildTree, type TreeNode} from "@/lib/file-tree"
import {FileIcon} from "@/lib/file-icon"
import {formatLineRef} from "@/lib/diff"
import {errorText} from "@/lib/utils"
import type {DocLineSelection} from "@/lib/codemirror"
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuTrigger,
} from "@/components/ui/context-menu"
import {useFileEditor} from "./useFileEditor"

// FilesPanel is the Files tab of the right dock: a read-only tree of the active
// session's tracked files, master-detail with an in-dock preview. It follows the
// active session like the review panel — a worktree session browses its
// checkout, not the project root — so clicking a file opens it beside the same
// terminal it belongs to. It never edits; clicks only navigate and inject
// path/line references into the session's PTY.
export function FilesPanel() {
  const {projectId, sessionId, path} = useActiveSession()
  const status = useGitStatus(path)
  const [tree, setTree] = useState<TreeNode[] | null>(null)
  const [failed, setFailed] = useState(false)
  const [open, setOpen] = useState<string | null>(null)

  const refresh = useCallback(async () => {
    if (!path) {
      return
    }
    try {
      const files = await ProjectService.Tree(path)
      setTree(buildTree(files ?? []))
      setFailed(false)
    } catch {
      setTree([])
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

  const inject = (text: string) => {
    if (sessionId) {
      void TerminalService.Write(sessionId, text)
    }
  }

  if (open !== null) {
    return (
      <FilePreview
        path={path}
        rel={open}
        onBack={() => setOpen(null)}
        onInject={inject}
      />
    )
  }
  return <TreeBody tree={tree} failed={failed} onOpen={setOpen}/>
}

interface TreeBodyProps {
  tree: TreeNode[] | null
  failed: boolean
  onOpen: (rel: string) => void
}

function TreeBody({tree, failed, onOpen}: TreeBodyProps) {
  const [expanded, setExpanded] = useState<Set<string>>(new Set())
  const toggle = (rel: string) =>
    setExpanded((prev) => {
      const next = new Set(prev)
      if (next.has(rel)) {
        next.delete(rel)
      } else {
        next.add(rel)
      }
      return next
    })

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
    <div role="tree" className="h-full overflow-y-auto py-1 font-mono text-xs">
      {tree.map((node) => (
        <TreeRow
          key={node.path}
          node={node}
          depth={0}
          expanded={expanded}
          onToggle={toggle}
          onOpen={onOpen}
        />
      ))}
    </div>
  )
}

interface TreeRowProps {
  node: TreeNode
  depth: number
  expanded: Set<string>
  onToggle: (rel: string) => void
  onOpen: (rel: string) => void
}

function TreeRow({node, depth, expanded, onToggle, onOpen}: TreeRowProps) {
  const isOpen = expanded.has(node.path)
  // Indent by depth; the base padding keeps even top-level rows off the edge.
  const indent = {paddingLeft: `${depth * 0.75 + 0.5}rem`}
  if (node.type === "file") {
    // A chevron-width spacer keeps file names aligned under their folder's name;
    // FileIcon draws the language's real logo (devicon).
    return (
      <button
        type="button"
        onClick={() => onOpen(node.path)}
        style={indent}
        title={node.path}
        className="flex items-center gap-1.5 py-0.5 pr-2 text-left text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground"
      >
        <span className="size-3.5 shrink-0" aria-hidden/>
        <FileIcon path={node.path}/>
        <span className="truncate">{node.name}</span>
      </button>
    )
  }
  const Chevron = isOpen ? ChevronDown : ChevronRight
  const FolderIcon = isOpen ? FolderOpen : Folder
  return (
    <>
      <button
        type="button"
        onClick={() => onToggle(node.path)}
        style={indent}
        aria-expanded={isOpen}
        className="flex items-center gap-1.5 py-0.5 pr-2 text-left font-medium transition-colors hover:bg-accent hover:text-accent-foreground"
      >
        <Chevron className="size-3.5 shrink-0 text-muted-foreground"/>
        <FolderIcon className="size-3.5 shrink-0 text-muted-foreground"/>
        <span className="truncate">{node.name}</span>
      </button>
      {isOpen &&
        node.children.map((child) => (
          <TreeRow
            key={child.path}
            node={child}
            depth={depth + 1}
            expanded={expanded}
            onToggle={onToggle}
            onOpen={onOpen}
          />
        ))}
    </>
  )
}

interface FilePreviewProps {
  path: string
  rel: string
  onBack: () => void
  onInject: (text: string) => void
}

function FilePreview({path, rel, onBack, onInject}: FilePreviewProps) {
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
          <ChevronLeft className="size-4"/>
        </button>
        <span className="truncate font-mono" title={rel}>
          {rel}
        </span>
        <span className="ml-auto shrink-0 rounded border border-border px-1 text-[0.5625rem] uppercase tracking-wide text-muted-foreground">
          read-only
        </span>
      </div>
      <div className="flex-1 overflow-y-auto">
        {error !== null ? (
          <Notice>{error}</Notice>
        ) : text === null ? (
          <Notice>Loading…</Notice>
        ) : (
          <PreviewBody text={text} rel={rel} onInject={onInject}/>
        )}
      </div>
    </div>
  )
}

interface PreviewBodyProps {
  text: string
  rel: string
  onInject: (text: string) => void
}

// PreviewBody renders the file in a read-only CodeMirror view whose selection
// drives the same inject context menu as the diff review — file lines map
// straight through (doc line === file line), so the range needs no remap.
function PreviewBody({text, rel, onInject}: PreviewBodyProps) {
  const {containerRef, getSelectedLines} = useFileEditor(text, rel)
  const [range, setRange] = useState<DocLineSelection | null>(null)

  // Resolve the selection when the menu opens, not on every selection change.
  const onOpenChange = (menuOpen: boolean) => {
    if (menuOpen) {
      setRange(getSelectedLines())
    }
  }

  const lineRef = range && formatLineRef({start: range.from, end: range.to})
  return (
    <ContextMenu onOpenChange={onOpenChange}>
      <ContextMenuTrigger render={<div className="isolate py-1" ref={containerRef}/>}/>
      <ContextMenuContent>
        <ContextMenuItem onClick={() => onInject(`@${rel} `)}>
          Inject file
        </ContextMenuItem>
        <ContextMenuItem
          disabled={lineRef === null}
          onClick={() => lineRef && onInject(`${rel}:${lineRef} `)}
        >
          {lineRef === null ? "Inject lines" : `Inject lines ${lineRef}`}
        </ContextMenuItem>
      </ContextMenuContent>
    </ContextMenu>
  )
}

function Notice({children}: {children: string}) {
  return <p className="px-3 py-4 text-xs text-muted-foreground">{children}</p>
}
