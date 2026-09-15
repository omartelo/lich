import { ChevronDown, ChevronRight, MessageSquare, Paperclip, Undo2 } from "lucide-react"
import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import { toast } from "sonner"
import { IconAction } from "@/components/common/IconAction"
import { DiffStat } from "@/components/DiffStat"
import { Checkbox } from "@/components/ui/checkbox"
import type { RevertLine } from "@/lib/api-types"
import {
  appendExpansion,
  buildFileDoc,
  type DiffFile,
  type DiffGap,
  type Expansions,
} from "@/lib/git/diff"
import { shownLayout } from "@/lib/git/diff-layout"
import { languageAbbr, splitPath } from "@/lib/git/lang-badge"
import { isAnchored } from "@/lib/pulls/review-slots"
import { useElementWidth } from "@/lib/use-element-width"
import { cn, errorText } from "@/lib/utils"
import { useSettings } from "@/providers/settings"
import { DiffBody, type DiffBodyProps } from "./DiffBody"
import type { DiffBulk } from "./diff-bulk"
import type { DiffReview } from "./ReviewSlots"
import { SplitDiffBody } from "./SplitDiffBody"

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

// One shared empty map, so a file nobody expanded holds no state of its own.
const NO_EXPANSIONS: Expansions = new Map()

interface FileDiffProps {
  file: DiffFile
  onInject: (text: string) => void
  /** Hold a comment on these lines for the panel's batch, which the session's
   * next prompt carries. Absent = no session comment, the shape every surface
   * had before review comments existed. */
  onSessionComment?: (path: string, lines: string, text: string) => void
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
  /** Read the file's unchanged lines around the hunks, from..to inclusive and
   * numbered against the new side. Absent = no expanders, for a panel whose
   * diff has no revision to read them from. */
  onExpand?: (path: string, from: number, to: number) => Promise<string[] | null>
  /** Put changed lines of this file back to HEAD. Absent wherever the diff is
   * not the working tree's own: a finished turn, a pull request. */
  onRevertLines?: (lines: RevertLine[]) => void
}

// The card must not clip overflow — a clipping ancestor would break the
// sticky header.
export function FileDiff({
  file,
  onInject,
  onSessionComment,
  onDiscard,
  bulk,
  viewed,
  onViewed,
  review,
  onExpand,
  onRevertLines,
}: FileDiffProps) {
  // What the reader has pulled into this file's gaps. Held here, above the
  // editor, because the document is built from it: an expanded gap is context
  // lines in the doc, never a second document cropped to a range.
  const [expansions, setExpansions] = useState<Expansions>(NO_EXPANSIONS)
  // The diff as git printed it, and the diff as it is being read. They are the
  // same object until something is expanded, so a file nobody expanded pays
  // nothing, and the identity check below stays on the printed one.
  const printed = useMemo(() => buildFileDoc(file), [file])
  const doc = useMemo(
    () => (expansions.size === 0 ? printed : buildFileDoc(file, expansions)),
    [file, printed, expansions],
  )
  const openByDefault = !file.binary && printed.lineMeta.length <= LARGE_FILE_LINES
  const [expanded, setExpanded] = useState(openByDefault)
  // The card outlives its content: the panel keys files by path, so a refetch
  // that rewrites this file reuses this component. Without the re-sync, a file
  // folded away by its Viewed tick stayed folded after a commit unticked it —
  // an unread file, closed, with nothing on screen saying why — and one that
  // grew past the large-file bar stayed open.
  //
  // Keyed on the text and not on the doc object: every refetch builds a fresh
  // one, so a reference check would reopen every hand-folded file on each
  // window focus. Only the content actually changing counts as a new file.
  const lastText = useRef(printed.text)
  useEffect(() => {
    if (lastText.current !== printed.text) {
      lastText.current = printed.text
      setExpanded(openByDefault)
      // The gaps moved with the content, and what was pulled into the old ones
      // is numbered against a file that no longer exists.
      setExpansions(NO_EXPANSIONS)
    }
  }, [printed.text, openByDefault])
  // The nonce guard skips the initial mount so each file keeps its own
  // large-file default until the user actually triggers a bulk action.
  const lastNonce = useRef(bulk?.nonce)
  useEffect(() => {
    if (bulk && bulk.nonce !== lastNonce.current) {
      lastNonce.current = bulk.nonce
      setExpanded(bulk.open)
    }
  }, [bulk])
  // The prop is read through a ref so the handler can be stable: it rides the
  // editor's identity through gapExpanders, and a new one per render would
  // rebuild every expanded file's view on every render of the panel.
  const latestExpand = useRef(onExpand)
  useEffect(() => {
    latestExpand.current = onExpand
  })
  const expandGap = useCallback(
    async (gap: DiffGap): Promise<void> => {
      const read = latestExpand.current
      if (!read) {
        return
      }
      try {
        const texts = await read(file.newPath, gap.from, gap.to)
        if (!texts || texts.length === 0) {
          return
        }
        setExpansions((held) => appendExpansion(held, gap, texts))
      } catch (error) {
        toast.error(`Couldn't read ${file.newPath}`, { description: errorText(error) })
      }
    },
    [file.newPath],
  )

  const card = useRef<HTMLElement>(null)
  const layout = shownLayout(useSettings().diffLayout, useElementWidth(card))
  const Chevron = expanded ? ChevronDown : ChevronRight
  const badge = languageAbbr(file.newPath)
  const { dir, base } = splitPath(file.newPath)
  // Only what opening the file would actually reveal: a settled thread is not
  // outstanding, and one the diff cannot anchor lives in the Conversation tab.
  const talking = review
    ? review.threads.filter((thread) => !thread.isResolved && isAnchored(thread, doc.lineMeta))
        .length + review.drafts.length
    : 0

  return (
    <section ref={card}>
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
          <LazyDiffBody
            key={layout}
            layout={layout}
            doc={doc}
            path={file.newPath}
            onInject={onInject}
            onSessionComment={onSessionComment}
            review={review}
            onExpandGap={onExpand && expandGap}
            // A rename's old side lives under another path, which the backend's
            // one-file revert cannot reach; Discard still reverts the pair.
            onRevertLines={file.status === "renamed" ? undefined : onRevertLines}
          />
        ))}
    </section>
  )
}

// A panel holds one CodeMirror view per expanded file, and a wide diff (a PR
// touching 191 files) built them all in a single commit — enough to lock the
// page. LazyDiffBody defers each one until its card comes near the viewport;
// once built, the editor stays, so scrolling back keeps its selection and
// highlighting. Collapsing the file still destroys it.
function LazyDiffBody({ layout, ...body }: DiffBodyProps & { layout: "unified" | "split" }) {
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
          // Its work is done, and the placeholder it watches is about to leave
          // the DOM — an observer holds its target, so without this the removed
          // node stays alive for as long as the panel does.
          observer.disconnect()
          setMounted(true)
        }
      },
      { rootMargin: MOUNT_MARGIN },
    )
    observer.observe(target)
    return () => observer.disconnect()
  }, [])

  if (mounted) {
    return layout === "split" ? <SplitDiffBody {...body} /> : <DiffBody {...body} />
  }
  // The placeholder has to stand in for the file's height: zero-height cards
  // would all sit in the viewport at once and nothing would be deferred.
  return <div ref={placeholder} style={{ height: body.doc.lineMeta.length * LINE_HEIGHT_PX }} />
}
