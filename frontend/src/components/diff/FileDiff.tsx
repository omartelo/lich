import { ChevronDown, ChevronRight, MessageSquare, Paperclip, Undo2 } from "lucide-react"
import { useEffect, useMemo, useRef, useState } from "react"
import { createPortal } from "react-dom"
import { IconAction } from "@/components/common/IconAction"
import { DiffStat } from "@/components/DiffStat"
import { Checkbox } from "@/components/ui/checkbox"
import { threadSlots, type SlotElements, type ThreadSlot } from "@/lib/codemirror-threads"
import {
  buildFileDoc,
  formatLineRef,
  newLineRange,
  type DiffFile,
  type FileDoc,
  type NewLineRange,
} from "@/lib/git/diff"
import { languageAbbr, splitPath } from "@/lib/git/lang-badge"
import { reviewSlots } from "@/lib/pulls/review-slots"
import { cn } from "@/lib/utils"
import type { DiffBulk } from "./diff-bulk"
import { InjectMenu } from "./InjectMenu"
import { useDiffEditor } from "./useDiffEditor"
import { type Composer, type DiffReview, ReviewSlot } from "./ReviewSlots"

// Files whose rendered diff exceeds this many lines start collapsed, so one
// giant lockfile doesn't swamp the panel (expanding is one click away).
const LARGE_FILE_LINES = 500

// Height of one editor line, the knob sizing a file's placeholder before its
// editor exists. Measured against the diff theme's 12px font; a wrapped long
// line makes any single number a lower bound, so this only has to be close.
const LINE_HEIGHT_PX = 17

// How far outside the viewport a file's editor is built, so scrolling meets a
// rendered diff rather than a gap.
const MOUNT_MARGIN = "600px 0px"

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
  /** The pull request review over this file: threads to render inline, drafts
   * waiting to be sent, and what a new comment does. Absent for the working
   * diff, which has no pull request behind it. */
  review?: DiffReview
}

// The card must not clip overflow — a clipping ancestor would break the
// sticky header.
export function FileDiff({
  file,
  onInject,
  onDiscard,
  bulk,
  viewed,
  onViewed,
  review,
}: FileDiffProps) {
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
  const talking = review
    ? review.threads.filter((thread) => !thread.isResolved).length + review.drafts.length
    : 0

  return (
    <section>
      <div
        className={cn(
          "sticky top-0 z-10 flex w-full items-center gap-2 bg-sidebar px-2 py-1 text-xs",
          expanded && "border-b border-border/60",
        )}
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
            className={cn(
              "flex size-5 shrink-0 items-center justify-center rounded text-[0.5625rem] font-bold",
              badge.className,
            )}
          >
            {badge.abbr}
          </span>
          <span className="truncate font-medium" title={file.newPath}>
            {file.status === "renamed" ? `${file.oldPath} → ${base}` : base}
          </span>
          {dir && <span className="truncate text-muted-foreground">{dir}</span>}
        </button>
        {/* A collapsed file must still say it is being talked about — the
            threads inside it are invisible until it is opened. Settled threads
            are left out of the count: they are not what is outstanding. */}
        {talking > 0 && (
          <span className="flex shrink-0 items-center gap-1 text-muted-foreground">
            <MessageSquare className="size-3.5" />
            <span className="tabular-nums">{talking}</span>
          </span>
        )}
        <span className="flex shrink-0 items-center gap-1.5">
          <DiffStat added={file.added} deleted={file.deleted} />
        </span>
        <IconAction label="Add file as context" onClick={() => onInject(`@${file.newPath} `)}>
          <Paperclip className="size-3.5" />
        </IconAction>
        {onDiscard && (
          <IconAction label="Discard Changes" onClick={onDiscard}>
            <Undo2 className="size-3.5" />
          </IconAction>
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
          <LazyDiffBody doc={doc} path={file.newPath} onInject={onInject} review={review} />
        ))}
    </section>
  )
}

interface DiffBodyProps {
  doc: FileDoc
  path: string
  onInject: (text: string) => void
  review?: DiffReview
}

// A panel holds one CodeMirror view per expanded file, and a wide diff (a PR
// touching 191 files) built them all in a single commit — enough to lock the
// page. LazyDiffBody defers each one until its card comes near the viewport;
// once built, the editor stays, so scrolling back keeps its selection and
// highlighting. Collapsing the file still destroys it.
function LazyDiffBody({ doc, path, onInject, review }: DiffBodyProps) {
  const placeholder = useRef<HTMLDivElement>(null)
  const [mounted, setMounted] = useState(false)

  useEffect(() => {
    const target = placeholder.current
    if (!target) {
      return
    }
    const observer = new IntersectionObserver(
      ([entry]) => {
        if (entry.isIntersecting) {
          setMounted(true)
        }
      },
      { rootMargin: MOUNT_MARGIN },
    )
    observer.observe(target)
    return () => observer.disconnect()
  }, [])

  if (mounted) {
    return <DiffBody doc={doc} path={path} onInject={onInject} review={review} />
  }
  // The placeholder has to stand in for the file's height: zero-height cards
  // would all sit in the viewport at once and nothing would be deferred.
  return <div ref={placeholder} style={{ height: doc.lineMeta.length * LINE_HEIGHT_PX }} />
}

const NO_SLOTS: SlotElements = new Map()

// DiffBody exists as its own component so collapsing the file unmounts it,
// destroying the CodeMirror view instead of keeping it alive off-screen.
//
// A diff's document is not the file: the selection lands on doc lines, which
// newLineRange maps back to the line numbers the agent needs to be told — and,
// under a pull request, the line numbers GitHub files a comment against.
function DiffBody({ doc, path, onInject, review }: DiffBodyProps) {
  // One slot extension per view. It is memoised because it *is* part of the
  // view's identity: a new one would rebuild the editor on every render.
  const [elements, setElements] = useState<SlotElements>(NO_SLOTS)
  const slots = useMemo(() => threadSlots(setElements), [])
  const { containerRef, getSelectedDocLines, view } = useDiffEditor(
    doc,
    path,
    review ? slots.extension : undefined,
  )
  const [range, setRange] = useState<NewLineRange | null>(null)
  const [composer, setComposer] = useState<Composer | null>(null)

  // Where the gaps go, never what is in them: keyed off the composer's line
  // rather than the composer, so typing into it does not re-dispatch the whole
  // slot set on every keystroke.
  const composerLine = composer?.end ?? 0
  const wanted = useMemo(
    (): ThreadSlot[] =>
      review
        ? reviewSlots({
            lineMeta: doc.lineMeta,
            threads: review.threads,
            drafts: review.drafts.map(({ comment }) => comment),
            composerLine,
          })
        : [],
    [review, doc.lineMeta, composerLine],
  )

  useEffect(() => {
    if (view && review) {
      slots.update(view, wanted)
    }
  }, [view, review, slots, wanted])

  const addComment = (): void => {
    if (!review || !composer || composer.body.trim() === "") {
      return
    }
    review.onAdd({
      path,
      line: composer.end,
      startLine: composer.start === composer.end ? 0 : composer.start,
      side: "RIGHT",
      body: composer.body.trim(),
    })
    setComposer(null)
  }

  return (
    <>
      <InjectMenu
        path={path}
        containerRef={containerRef}
        lineRef={range && formatLineRef(range)}
        // Resolve the selection when the menu opens, not on every change.
        onOpenChange={(open) => {
          if (!open) {
            return
          }
          const selected = getSelectedDocLines()
          setRange(selected ? newLineRange(doc.lineMeta, selected.from, selected.to) : null)
        }}
        onInject={onInject}
        // Commenting only exists under a pull request — but the item is offered
        // whenever there is one, not only once a range is resolved: the
        // selection is read as the menu opens, so gating the item on it would
        // hide it on the very open that produced it. With nothing selected the
        // menu disables it, like Inject lines beside it.
        //
        // Deliberately new-file lines only: a comment on a deleted line is a
        // thread GitHub anchors on the other side, and that needs its own gesture.
        onComment={
          review
            ? () => range && setComposer({ start: range.start, end: range.end, body: "" })
            : undefined
        }
      />
      {review &&
        [...elements].map(([key, element]) =>
          createPortal(
            <ReviewSlot
              slotKey={key}
              review={review}
              composer={composer}
              onComposerChange={(body) => setComposer((held) => held && { ...held, body })}
              onComposerSubmit={addComment}
              onComposerCancel={() => setComposer(null)}
            />,
            element,
            key,
          ),
        )}
    </>
  )
}
