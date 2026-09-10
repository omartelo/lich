import { useState } from "react"
import { ChevronDown, ChevronRight, Folder, FolderOpen } from "lucide-react"
import type { TreeNode } from "@/lib/git/file-tree"
import type { DiffFile } from "@/lib/git/diff"
import { FileIcon } from "./FileIcon"
import { DiffStat } from "./DiffStat"
import { cn } from "@/lib/utils"
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuTrigger,
} from "@/components/ui/context-menu"

interface FileTreeProps {
  tree: TreeNode[]
  /** Path of the file to highlight; null for no selection. */
  active?: string | null
  /** Start folders expanded — a pull request's changed files, where every row
   * is part of the review. The repo browser starts collapsed instead. */
  defaultOpen?: boolean
  /** Hold every folder open, whatever was toggled by hand — a filtered tree,
   * where a match has to be visible without opening its ancestors first. The
   * toggles are only suspended, so clearing the filter restores the browse. */
  expandAll?: boolean
  /** The folders toggled *away* from `defaultOpen`, for a caller that has to
   * outlive its own tree: the dock's browser is unmounted on every flip to the
   * Review tab, so its folds are held outside it (file-browse). Left out — the
   * pull request's tree, which lives as long as the tab it is on — the tree
   * holds them itself and they go when it does. */
  toggled?: ReadonlySet<string>
  onToggled?: (toggled: ReadonlySet<string>) => void
  /** Per-file diff counts keyed by path; a row with an entry shows its +/-. */
  stats?: Map<string, DiffFile>
  /** Right-click → Open in editor. Absent = no file context menu, which is the
   * pull request's tree: what it lists is the PR's diff, not what sits on disk. */
  onEditor?: (path: string) => void
  className?: string
  onSelect: (path: string) => void
}

// FileTree renders a buildTree() result as a collapsible folder/file tree. The
// dock's repo browser and the pull request's changed-files navigator are the
// same widget: they differ only in what a click does, whether folders start
// open, and whether a row reads as selected.
export function FileTree({
  tree,
  active = null,
  defaultOpen = false,
  expandAll = false,
  toggled: lifted,
  onToggled: onLifted,
  stats,
  onEditor,
  className,
  onSelect,
}: FileTreeProps) {
  // The set holds the rows toggled *away* from defaultOpen, so a folder's state
  // survives its parent closing and reopening.
  const [own, setOwn] = useState<ReadonlySet<string>>(() => new Set())
  const toggled = lifted ?? own
  const publish = onLifted ?? setOwn
  const isOpen = (path: string) => expandAll || toggled.has(path) !== defaultOpen

  const toggle = (path: string) => {
    const next = new Set(toggled)
    if (!next.delete(path)) {
      next.add(path)
    }
    publish(next)
  }

  // Expand/collapse a directory and every directory beneath it, the scope a
  // right-click on that folder implies. Entries are deviations from defaultOpen,
  // so "expand" adds or removes depending on which way that default points.
  const setSubtree = (node: TreeNode, expand: boolean) => {
    const next = new Set(toggled)
    const walk = (n: TreeNode) => {
      if (n.type !== "dir") {
        return
      }
      if (expand === defaultOpen) {
        next.delete(n.path)
      } else {
        next.add(n.path)
      }
      n.children.forEach(walk)
    }
    walk(node)
    publish(next)
  }

  return (
    // Names run past the panel's width on a deep tree; the rows size to their
    // content and the tree scrolls sideways, because an ellipsis with no way to
    // reach the rest of the name hides which file the row is.
    <div className={cn("overflow-auto", className)}>
      <div role="tree" className="w-max min-w-full py-1 font-mono text-xs">
        {tree.map((node) => (
          <TreeRow
            key={node.path}
            node={node}
            depth={0}
            active={active}
            stats={stats}
            isOpen={isOpen}
            onToggle={toggle}
            onSubtree={setSubtree}
            onEditor={onEditor}
            onSelect={onSelect}
          />
        ))}
      </div>
    </div>
  )
}

interface TreeRowProps {
  node: TreeNode
  depth: number
  active: string | null
  stats: Map<string, DiffFile> | undefined
  isOpen: (path: string) => boolean
  onToggle: (path: string) => void
  onSubtree: (node: TreeNode, expand: boolean) => void
  onEditor: ((path: string) => void) | undefined
  onSelect: (path: string) => void
}

function TreeRow({
  node,
  depth,
  active,
  stats,
  isOpen,
  onToggle,
  onSubtree,
  onEditor,
  onSelect,
}: TreeRowProps) {
  // The 0.5rem base keeps even top-level rows off the edge.
  const indent = { paddingLeft: `${depth * 0.75 + 0.5}rem` }
  if (node.type === "file") {
    const stat = stats?.get(node.path)
    // A chevron-width spacer keeps file names aligned under their folder's name;
    // FileIcon draws the language's real logo (devicon).
    const row = (
      <>
        <span className="size-3.5 shrink-0" aria-hidden />
        <FileIcon path={node.path} />
        <span className="whitespace-nowrap">{node.name}</span>
        {stat && (
          <span className="ml-auto flex shrink-0 items-center gap-1.5 pl-2 tabular-nums">
            <DiffStat added={stat.added} deleted={stat.deleted} />
          </span>
        )}
      </>
    )
    const className = cn(
      "flex w-full items-center gap-1.5 py-0.5 pr-2 text-left text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground",
      node.path === active && "bg-accent text-accent-foreground",
    )
    if (!onEditor) {
      return (
        <button
          type="button"
          onClick={() => onSelect(node.path)}
          style={indent}
          title={node.path}
          className={className}
        >
          {row}
        </button>
      )
    }
    return (
      <ContextMenu>
        <ContextMenuTrigger
          render={
            <button
              type="button"
              onClick={() => onSelect(node.path)}
              style={indent}
              title={node.path}
              className={className}
            />
          }
        >
          {row}
        </ContextMenuTrigger>
        <ContextMenuContent>
          <ContextMenuItem onClick={() => onEditor(node.path)}>Open in editor</ContextMenuItem>
        </ContextMenuContent>
      </ContextMenu>
    )
  }
  const open = isOpen(node.path)
  const Chevron = open ? ChevronDown : ChevronRight
  const FolderIcon = open ? FolderOpen : Folder
  return (
    <>
      <ContextMenu>
        <ContextMenuTrigger
          render={
            <button
              type="button"
              onClick={() => onToggle(node.path)}
              style={indent}
              aria-expanded={open}
              className="flex w-full items-center gap-1.5 py-0.5 pr-2 text-left font-medium transition-colors hover:bg-accent hover:text-accent-foreground"
            />
          }
        >
          <Chevron className="size-3.5 shrink-0 text-muted-foreground" />
          <FolderIcon className="size-3.5 shrink-0 text-muted-foreground" />
          <span className="whitespace-nowrap">{node.name}</span>
        </ContextMenuTrigger>
        <ContextMenuContent>
          <ContextMenuItem onClick={() => onSubtree(node, true)}>Expand all</ContextMenuItem>
          <ContextMenuItem onClick={() => onSubtree(node, false)}>Collapse all</ContextMenuItem>
        </ContextMenuContent>
      </ContextMenu>
      {open &&
        node.children.map((child) => (
          <TreeRow
            key={child.path}
            node={child}
            depth={depth + 1}
            active={active}
            stats={stats}
            isOpen={isOpen}
            onToggle={onToggle}
            onSubtree={onSubtree}
            onEditor={onEditor}
            onSelect={onSelect}
          />
        ))}
    </>
  )
}
