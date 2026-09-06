import type { ReactNode } from "react"
import { ArrowUpRight, ChevronRight, Info, TriangleAlert } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogFooter, DialogTitle } from "@/components/ui/dialog"
import { System } from "@/lib/rpc"
import type { PatchNotes, PatchNotesHighlight } from "@/lib/api-types"
import { layoutHighlights, splitLeadIn } from "@/lib/update/patch-notes-layout"
import { cn } from "@/lib/utils"

const RELEASE_TAG_BASE = "https://github.com/omartelo/lich/releases/tag/v"

// Semantic per-group accents, deliberately separate from the app accent.
function dotColor(label: string): string {
  switch (label.toLowerCase()) {
    case "added":
      return "bg-emerald-500"
    case "changed":
      return "bg-amber-500"
    case "fixed":
      return "bg-sky-500"
    default:
      return "bg-muted-foreground"
  }
}

// renderInline renders a changelog item's markdown: **bold** lead-ins and
// `code` spans. No full markdown parser — those two are all the CHANGELOG uses.
// The split is positional and static for a given item, so the index is the key.
function renderInline(text: string): ReactNode[] {
  return text.split(/(\*\*[^*]+\*\*|`[^`]+`)/g).map((part, i) => {
    if (part.startsWith("**") && part.endsWith("**")) {
      return (
        // biome-ignore lint/suspicious/noArrayIndexKey: static split, never reordered.
        <strong key={i} className="font-medium text-foreground">
          {part.slice(2, -2)}
        </strong>
      )
    }
    if (part.startsWith("`") && part.endsWith("`")) {
      return (
        // biome-ignore lint/suspicious/noArrayIndexKey: static split, never reordered.
        <code key={i} className="rounded bg-accent px-1 py-0.5 font-mono text-[0.85em]">
          {part.slice(1, -1)}
        </code>
      )
    }
    return part
  })
}

// The release's headline: the first IMPORTANT block, set as the largest type
// the app uses and nothing else — no fill, no badge. Size and space carry it.
function Lead({ highlight }: { highlight: PatchNotesHighlight }) {
  const { headline, body } = splitLeadIn(highlight.text)
  return (
    <div className="border-b px-6 pt-5 pb-6">
      <p className="max-w-[22ch] text-2xl leading-tight font-semibold tracking-tight text-balance">
        {renderInline(headline)}
      </p>
      {body !== "" && (
        <p className="mt-2.5 max-w-[56ch] text-sm leading-relaxed text-muted-foreground">
          {renderInline(body)}
        </p>
      )}
    </div>
  )
}

// Every other block: a compact callout. A warning is the one that paints —
// the destructive tint says "do this or avoid this"; a note sits on the accent
// fill like a row.
function Callout({ highlight }: { highlight: PatchNotesHighlight }) {
  const warning = highlight.kind === "warning"
  const Icon = warning ? TriangleAlert : Info
  return (
    <div
      className={cn(
        "flex gap-2.5 rounded-md px-3 py-2.5 text-[0.8125rem] leading-relaxed text-muted-foreground",
        warning ? "bg-destructive/10 ring-1 ring-destructive/30" : "bg-accent/60",
      )}
    >
      <Icon
        className={cn(
          "mt-0.5 size-4 flex-none",
          warning ? "text-destructive" : "text-muted-foreground",
        )}
      />
      <div>{renderInline(highlight.text)}</div>
    </div>
  )
}

// One changelog item as a row: its bold lead-in, with the paragraph behind it
// opening on click. A native <details> — the row needs no state of its own.
function Item({ item }: { item: string }) {
  const { headline, body } = splitLeadIn(item)
  const row = "-mx-2 flex items-start gap-2 rounded-md px-2 py-1.5 text-[0.8125rem] leading-snug"
  if (body === "") {
    return (
      <div className={row}>
        <span className="mt-0.5 size-3.5 flex-none" />
        <span className="font-medium">{renderInline(headline)}</span>
      </div>
    )
  }
  return (
    <details className="group">
      <summary
        className={cn(
          row,
          "cursor-pointer list-none hover:bg-accent/50 [&::-webkit-details-marker]:hidden",
        )}
      >
        <ChevronRight className="mt-0.5 size-3.5 flex-none text-muted-foreground transition-transform group-open:rotate-90" />
        <span className="font-medium">{renderInline(headline)}</span>
      </summary>
      <p className="max-w-[60ch] pb-2 pl-5.5 text-[0.8125rem] leading-relaxed text-muted-foreground">
        {renderInline(body)}
      </p>
    </details>
  )
}

interface PatchNotesDialogProps {
  notes: PatchNotes
  /** Dismiss — also fired on Escape/backdrop; the gate records the version. */
  onClose: () => void
}

export function PatchNotesDialog({ notes, onClose }: PatchNotesDialogProps) {
  const groups = notes.groups ?? []
  const { lead, callouts } = layoutHighlights(notes.highlights)
  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="gap-0 overflow-hidden p-0 sm:max-w-xl" showCloseButton={false}>
        <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1 px-6 pt-5 text-[0.8125rem] text-muted-foreground">
          <DialogTitle className="text-[0.8125rem] font-medium text-foreground">
            What's new in lich <span className="font-normal tabular-nums">v{notes.version}</span>
          </DialogTitle>
          <div className="flex gap-3 tabular-nums">
            {groups.map((group) => (
              <span key={group.label} className="inline-flex items-center gap-1.5">
                <span className={cn("size-1.5 rounded-full", dotColor(group.label))} />
                {group.items.length} {group.label.toLowerCase()}
              </span>
            ))}
          </div>
        </div>

        {lead && <Lead highlight={lead} />}

        <div className="max-h-[60vh] overflow-y-auto px-6 pb-2">
          {callouts.length > 0 && (
            <div className="flex flex-col gap-2 pt-4">
              {callouts.map((h, i) => (
                // biome-ignore lint/suspicious/noArrayIndexKey: a release's blocks are read-only and never reordered.
                <Callout key={i} highlight={h} />
              ))}
            </div>
          )}
          {groups.map((group) => (
            <div key={group.label} className="border-t pt-3.5 pb-1.5 first:border-t-0">
              <div className="mb-1.5 flex items-center gap-1.5 text-xs font-semibold tracking-wide text-muted-foreground uppercase">
                <span className={cn("size-2 rounded-full", dotColor(group.label))} />
                {group.label}
              </div>
              {group.items.map((item, i) => (
                // biome-ignore lint/suspicious/noArrayIndexKey: a release's notes are read-only and never reordered.
                <Item key={i} item={item} />
              ))}
            </div>
          ))}
        </div>

        <DialogFooter className="flex-row items-center justify-between border-t p-6 pt-4 sm:justify-between">
          <button
            type="button"
            className="inline-flex items-center gap-1 text-[0.8125rem] text-muted-foreground hover:text-foreground"
            onClick={() => void System.OpenExternal(RELEASE_TAG_BASE + notes.version)}
          >
            View full changelog
            <ArrowUpRight className="size-3.5" />
          </button>
          <Button size="sm" onClick={onClose}>
            Got it
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
