import type {ReactNode} from "react"
import {ArrowUpRight, Sparkles} from "lucide-react"
import {Button} from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {System} from "@/lib/rpc"
import type {PatchNotes} from "@/lib/api-types"

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
function renderInline(text: string): ReactNode[] {
  return text.split(/(\*\*[^*]+\*\*|`[^`]+`)/g).map((part, i) => {
    if (part.startsWith("**") && part.endsWith("**")) {
      return (
        <strong key={i} className="font-medium text-foreground">
          {part.slice(2, -2)}
        </strong>
      )
    }
    if (part.startsWith("`") && part.endsWith("`")) {
      return (
        <code key={i} className="rounded bg-accent px-1 py-0.5 font-mono text-[0.85em]">
          {part.slice(1, -1)}
        </code>
      )
    }
    return part
  })
}

interface PatchNotesDialogProps {
  notes: PatchNotes
  /** Dismiss — also fired on Escape/backdrop; the gate records the version. */
  onClose: () => void
}

export function PatchNotesDialog({notes, onClose}: PatchNotesDialogProps) {
  const groups = notes.groups ?? []
  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="gap-0 overflow-hidden p-0 sm:max-w-lg">
        <DialogHeader className="flex-row items-start gap-3 p-6 pr-12 pb-4">
          <span className="grid size-10 flex-none place-items-center rounded-lg bg-emerald-500/15 text-emerald-500 ring-1 ring-emerald-500/30">
            <Sparkles className="size-5" />
          </span>
          <div className="min-w-0 flex-1 pt-0.5">
            <div className="flex flex-wrap items-center gap-2">
              <DialogTitle className="text-[1.0625rem]">What's new in lich</DialogTitle>
              <span className="flex-none rounded-full border bg-accent px-2 py-0.5 text-xs font-medium tabular-nums">
                v{notes.version}
              </span>
            </div>
            <DialogDescription className="mt-1">
              You updated to a new version — here's what changed.
            </DialogDescription>
          </div>
        </DialogHeader>

        <div className="max-h-[55vh] overflow-y-auto px-6">
          {groups.map((group) => (
            <div key={group.label} className="border-t py-3.5 first:border-t-0 first:pt-1">
              <div className="mb-2.5 flex items-center gap-1.5 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                <span className={`size-2 rounded-full ${dotColor(group.label)}`} />
                {group.label}
              </div>
              <ul className="flex flex-col gap-2.5">
                {group.items.map((item, i) => (
                  <li
                    key={i}
                    className="relative pl-3.5 text-[0.8125rem] leading-relaxed text-muted-foreground before:absolute before:top-2 before:left-0 before:size-1 before:rounded-full before:bg-border"
                  >
                    {renderInline(item)}
                  </li>
                ))}
              </ul>
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
