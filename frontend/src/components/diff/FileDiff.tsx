import { useEffect, useMemo, useRef, useState } from "react"
import type { ReactNode } from "react"
import { ChevronDown, ChevronRight, Paperclip, Undo2 } from "lucide-react"
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuTrigger,
} from "@/components/ui/context-menu"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import { Checkbox } from "@/components/ui/checkbox"
import {
  buildFileDoc,
  formatLineRef,
  newLineRange,
  type DiffFile,
  type FileDoc,
  type NewLineRange,
} from "@/lib/diff"
import { languageAbbr, splitPath } from "@/lib/lang-badge"
import { cn } from "@/lib/utils"
import { DiffStat } from "@/components/DiffStat"
import { useDiffEditor } from "./useDiffEditor"

// Files whose rendered diff exceeds this many lines start collapsed, so one
// giant lockfile doesn't swamp the panel (expanding is one click away).
const LARGE_FILE_LINES = 500

// A collapse/expand-all directive shared by every file in the panel. The nonce
// is bumped on each bulk action so files re-sync even to a target they already
// hold.
export interface DiffBulk {
  open: boolean
  nonce: number
}

interface FileDiffProps {
  file: DiffFile
  onInject: (text: string) => void
  /** Ask the panel to confirm and revert this file's changes. Omitted for a
   * read-only diff (a PR's changes), where discarding makes no sense. */
  onDiscard?: () => void
  /** Collapse/expand-all directive from the panel; absent = no bulk control. */
  bulk?: DiffBulk
  /** Whether the reviewer has ticked this file off. */
  viewed?: boolean
  /** Handle the tick. Absent = no Viewed control (the dock's working diff,
   * where every file is the current one). */
  onViewed?: (next: boolean) => void
}

// The card must not clip overflow — a clipping ancestor would break the
// sticky header.
export function FileDiff({ file, onInject, onDiscard, bulk, viewed, onViewed }: FileDiffProps) {
  const doc = useMemo(() => buildFileDoc(file), [file])
  const [expanded, setExpanded] = useState(!file.binary && doc.lineMeta.length <= LARGE_FILE_LINES)
  // The nonce guard skips the initial mount so each file keeps its own
  // large-file default until the user actually triggers a bulk action.
  const lastNonce = useRef(bulk?.nonce)
  useEffect(() => {
    if (bulk && bulk.nonce !== lastNonce.current) {
      lastNonce.current = bulk.nonce
      setExpanded(bulk.open)
    }
  }, [bulk])
  const Chevron = expanded ? ChevronDown : ChevronRight
  const badge = languageAbbr(file.newPath)
  const { dir, base } = splitPath(file.newPath)

  return (
    <section>
      <div
        className={`sticky top-0 z-10 flex w-full items-center gap-2 bg-sidebar px-2 py-1 text-xs ${
          expanded ? "border-b border-border/60" : ""
        }`}
      >
        <button
          type="button"
          onClick={() => setExpanded((v) => !v)}
          className={cn(
            "flex min-w-0 flex-1 items-center gap-2 rounded-md py-0.5 transition-colors hover:text-foreground",
            // A ticked-off file recedes without leaving the list.
            viewed && "opacity-60",
          )}
        >
          <Chevron className="size-3.5 shrink-0 text-muted-foreground" />
          <span
            className={`flex size-5 shrink-0 items-center justify-center rounded text-[0.5625rem] font-bold ${badge.className}`}
          >
            {badge.abbr}
          </span>
          <span className="truncate font-medium" title={file.newPath}>
            {file.status === "renamed" ? `${file.oldPath} → ${base}` : base}
          </span>
          {dir && <span className="truncate text-muted-foreground">{dir}</span>}
        </button>
        <span className="flex shrink-0 items-center gap-1.5">
          <DiffStat added={file.added} deleted={file.deleted} />
        </span>
        <HeaderAction label="Add file as context" onClick={() => onInject(`@${file.newPath} `)}>
          <Paperclip className="size-3.5" />
        </HeaderAction>
        {onDiscard && (
          <HeaderAction label="Discard Changes" onClick={onDiscard}>
            <Undo2 className="size-3.5" />
          </HeaderAction>
        )}
        {onViewed && (
          <label
            htmlFor={`viewed-${file.newPath}`}
            className="flex shrink-0 cursor-pointer select-none items-center gap-1.5 pl-1 text-muted-foreground transition-colors hover:text-foreground"
          >
            <Checkbox
              id={`viewed-${file.newPath}`}
              checked={viewed ?? false}
              // Ticking a file folds it away; unticking brings it back, so the
              // checkbox doubles as the "done with this one" gesture.
              onCheckedChange={(next) => {
                onViewed(next)
                setExpanded(!next)
              }}
            />
            Viewed
          </label>
        )}
      </div>
      {expanded &&
        (file.binary ? (
          <p className="px-9 py-2 text-xs text-muted-foreground">Binary file</p>
        ) : (
          <DiffBody doc={doc} path={file.newPath} onInject={onInject} />
        ))}
    </section>
  )
}

interface HeaderActionProps {
  label: string
  onClick: () => void
  children: ReactNode
}

export function HeaderAction({ label, onClick, children }: HeaderActionProps) {
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
function DiffBody({ doc, path, onInject }: DiffBodyProps) {
  const { containerRef, getSelectedDocLines } = useDiffEditor(doc, path)
  const [range, setRange] = useState<NewLineRange | null>(null)

  // Resolve the selection when the menu opens, not on every selection change.
  const onOpenChange = (open: boolean) => {
    if (!open) {
      return
    }
    const selected = getSelectedDocLines()
    setRange(selected ? newLineRange(doc.lineMeta, selected.from, selected.to) : null)
  }

  return (
    <ContextMenu onOpenChange={onOpenChange}>
      <ContextMenuTrigger render={<div className="isolate py-1" ref={containerRef} />} />
      <ContextMenuContent>
        <ContextMenuItem onClick={() => onInject(`@${path} `)}>Inject file</ContextMenuItem>
        <ContextMenuItem
          disabled={range === null}
          onClick={() => range && onInject(`${path}:${formatLineRef(range)} `)}
        >
          {range === null ? "Inject lines" : `Inject lines ${formatLineRef(range)}`}
        </ContextMenuItem>
      </ContextMenuContent>
    </ContextMenu>
  )
}
