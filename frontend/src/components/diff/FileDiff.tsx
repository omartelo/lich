import {useMemo, useState} from "react"
import type {ReactNode} from "react"
import {ChevronDown, ChevronRight, Paperclip, Undo2} from "lucide-react"
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuTrigger,
} from "@/components/ui/context-menu"
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import {
  buildFileDoc,
  formatLineRef,
  newLineRange,
  type DiffFile,
  type FileDoc,
  type NewLineRange,
} from "@/lib/diff"
import {languageAbbr, splitPath} from "@/lib/lang-badge"
import {DiffStat} from "@/components/DiffStat"
import {useDiffEditor} from "./useDiffEditor"

// Files whose rendered diff exceeds this many lines start collapsed, so one
// giant lockfile doesn't swamp the panel (expanding is one click away).
const LARGE_FILE_LINES = 500

interface FileDiffProps {
  file: DiffFile
  onInject: (text: string) => void
  /** Ask the panel to confirm and revert this file's changes. */
  onDiscard: () => void
}

// FileDiff is one card of the review panel: a sticky header with a language
// badge, the file name, +/- counts and per-file actions, and a read-only
// CodeMirror diff below. The card must not clip overflow — a clipping
// ancestor would break the sticky header.
export function FileDiff({file, onInject, onDiscard}: FileDiffProps) {
  const doc = useMemo(() => buildFileDoc(file), [file])
  const [expanded, setExpanded] = useState(
    !file.binary && doc.lineMeta.length <= LARGE_FILE_LINES,
  )
  const Chevron = expanded ? ChevronDown : ChevronRight
  const badge = languageAbbr(file.newPath)
  const {dir, base} = splitPath(file.newPath)

  return (
    <section className="rounded-lg border border-border bg-card">
      <div
        className={`sticky top-0 z-10 flex w-full items-center gap-2 rounded-t-lg bg-card px-2 py-1 text-xs ${
          expanded ? "border-b border-border" : "rounded-b-lg"
        }`}
      >
        <button
          type="button"
          onClick={() => setExpanded((v) => !v)}
          className="flex min-w-0 flex-1 items-center gap-2 rounded-md py-0.5 transition-colors hover:text-foreground"
        >
          <Chevron className="size-3.5 shrink-0 text-muted-foreground"/>
          <span
            className={`flex size-5 shrink-0 items-center justify-center rounded text-[0.5625rem] font-bold ${badge.className}`}
          >
            {badge.abbr}
          </span>
          <span className="truncate font-medium" title={file.newPath}>
            {file.status === "renamed" ? `${file.oldPath} → ${base}` : base}
          </span>
          {dir && (
            <span className="truncate text-muted-foreground">{dir}</span>
          )}
        </button>
        <span className="flex shrink-0 items-center gap-1.5">
          <DiffStat added={file.added} deleted={file.deleted}/>
        </span>
        <HeaderAction label="Add file as context" onClick={() => onInject(`@${file.newPath} `)}>
          <Paperclip className="size-3.5"/>
        </HeaderAction>
        <HeaderAction label="Discard Changes" onClick={onDiscard}>
          <Undo2 className="size-3.5"/>
        </HeaderAction>
      </div>
      {expanded &&
        (file.binary ? (
          <p className="px-9 py-2 text-xs text-muted-foreground">Binary file</p>
        ) : (
          <DiffBody doc={doc} path={file.newPath} onInject={onInject}/>
        ))}
    </section>
  )
}

interface HeaderActionProps {
  label: string
  onClick: () => void
  children: ReactNode
}

// HeaderAction is one tooltip-labeled icon button in a review-panel header,
// styled after the footer's attach-file button.
export function HeaderAction({label, onClick, children}: HeaderActionProps) {
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <button
            type="button"
            onClick={onClick}
            aria-label={label}
            className="flex size-6 shrink-0 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground"
          />
        }
      >
        {children}
      </TooltipTrigger>
      <TooltipContent>{label}</TooltipContent>
    </Tooltip>
  )
}

interface DiffBodyProps {
  doc: FileDoc
  path: string
  onInject: (text: string) => void
}

// DiffBody exists as its own component so collapsing the file unmounts it,
// destroying the CodeMirror view instead of keeping it alive off-screen. The
// isolate wrapper keeps CodeMirror's high-z-index gutter from painting over
// the sticky card header.
function DiffBody({doc, path, onInject}: DiffBodyProps) {
  const {containerRef, getSelectedDocLines} = useDiffEditor(doc, path)
  const [range, setRange] = useState<NewLineRange | null>(null)

  // Resolve the selection when the menu opens, not on every selection change.
  const onOpenChange = (open: boolean) => {
    if (!open) {
      return
    }
    const selected = getSelectedDocLines()
    setRange(
      selected ? newLineRange(doc.lineMeta, selected.from, selected.to) : null,
    )
  }

  return (
    <ContextMenu onOpenChange={onOpenChange}>
      <ContextMenuTrigger render={<div className="isolate py-1" ref={containerRef}/>}/>
      <ContextMenuContent>
        <ContextMenuItem onClick={() => onInject(`@${path} `)}>
          Inject file
        </ContextMenuItem>
        <ContextMenuItem
          disabled={range === null}
          onClick={() => range && onInject(`${path}:${formatLineRef(range)} `)}
        >
          {range === null
            ? "Inject lines"
            : `Inject lines ${formatLineRef(range)}`}
        </ContextMenuItem>
      </ContextMenuContent>
    </ContextMenu>
  )
}
