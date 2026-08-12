import { useEffect, useRef } from "react"
import type { KeyboardEvent, ReactNode } from "react"
import { Dialog as DialogPrimitive } from "@base-ui/react/dialog"
import { CornerDownLeft, Search } from "lucide-react"
import { Keys } from "@/components/common/Keys"
import { cn } from "@/lib/utils"

// The one centered search-and-pick surface in the app: a query at the top, rows
// grouped under headings, a hint bar at the foot. Shared by the command palette
// (jump anywhere) and the delegate picker (hand this session's work to another)
// so the two read as the same control at a different scope — the second was a
// copy of the first, down to the popup's own geometry, and the copy is what
// let them drift.
//
// It owns the chrome and the keyboard's landing spot; what a row is, how many
// there are and what Enter does stay with the caller, which is where the
// selection index lives.

interface PickerDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** Names the dialog itself; read out on open, never drawn. */
  title: string
  placeholder: string
  /** Names the query field, which is not the same thing the dialog is called. */
  searchLabel: string
  /** Names the list the arrow keys walk. */
  resultsLabel: string
  query: string
  onQueryChange: (query: string) => void
  onKeyDown: (event: KeyboardEvent) => void
  /** What Enter does, for the hint bar: "open", "pick". */
  actionHint: string
  children: ReactNode
}

export function PickerDialog({
  open,
  onOpenChange,
  title,
  placeholder,
  searchLabel,
  resultsLabel,
  query,
  onQueryChange,
  onKeyDown,
  actionHint,
  children,
}: PickerDialogProps) {
  return (
    <DialogPrimitive.Root open={open} onOpenChange={onOpenChange}>
      <DialogPrimitive.Portal>
        <DialogPrimitive.Backdrop className="fixed inset-0 z-50 bg-black/50 supports-backdrop-filter:backdrop-blur-xs data-open:animate-in data-open:fade-in-0 data-closed:animate-out data-closed:fade-out-0" />
        <DialogPrimitive.Popup className="fixed left-1/2 top-[14vh] z-50 flex max-h-[70vh] w-full max-w-[40rem] -translate-x-1/2 flex-col overflow-hidden rounded-xl border bg-popover text-popover-foreground shadow-2xl ring-1 ring-foreground/10 outline-none data-open:animate-in data-open:fade-in-0 data-open:zoom-in-95 data-closed:animate-out data-closed:fade-out-0">
          <DialogPrimitive.Title className="sr-only">{title}</DialogPrimitive.Title>

          <div className="flex items-center gap-3 border-b px-4 py-3">
            <Search className="size-4 shrink-0 text-muted-foreground" />
            <input
              // biome-ignore lint/a11y/noAutofocus: the surface opens to be typed into right away.
              autoFocus
              value={query}
              onChange={(e) => onQueryChange(e.target.value)}
              onKeyDown={onKeyDown}
              placeholder={placeholder}
              aria-label={searchLabel}
              autoComplete="off"
              spellCheck={false}
              className="flex-1 bg-transparent text-[0.9375rem] outline-none placeholder:text-muted-foreground"
            />
          </div>

          {/* listbox/option, not bare buttons: `aria-selected` is meaningless
              on a button's implicit role, so the row the arrow keys are on was
              painted for the eye and announced to nobody. */}
          <div role="listbox" aria-label={resultsLabel} className="flex-1 overflow-y-auto p-1.5">
            {children}
          </div>

          <div className="flex items-center gap-4 border-t bg-black/10 px-4 py-2 text-xs text-muted-foreground">
            <Hint keys={["↑", "↓"]}>navigate</Hint>
            <Hint keys={["↵"]}>{actionHint}</Hint>
            <Hint keys={["esc"]}>close</Hint>
          </div>
        </DialogPrimitive.Popup>
      </DialogPrimitive.Portal>
    </DialogPrimitive.Root>
  )
}

// A listbox takes only options and groups as children, so a section heading
// rides inside a group rather than sitting loose between the rows. The heading
// can be hidden while the group still names itself to a screen reader — which
// is what a lone project does: nothing to tell apart, still worth announcing.
export function PickerGroup({
  label,
  showLabel = true,
  children,
}: {
  label: string
  showLabel?: boolean
  children: ReactNode
}) {
  return (
    // biome-ignore lint/a11y/useSemanticElements: the rule offers <fieldset>, which groups form controls and is not a valid child of a listbox.
    <div role="group" aria-label={label}>
      {showLabel && (
        <div className="px-3 pb-1 pt-3 text-[0.65625rem] font-semibold uppercase tracking-wider text-muted-foreground">
          {label}
        </div>
      )}
      {children}
    </div>
  )
}

export function PickerEmpty({ children }: { children: ReactNode }) {
  return <div className="px-3 py-8 text-center text-sm text-muted-foreground">{children}</div>
}

interface PickerRowProps {
  selected: boolean
  onSelect: () => void
  onRun: () => void
  children: ReactNode
}

export function PickerRow({ selected, onSelect, onRun, children }: PickerRowProps) {
  const ref = useRef<HTMLButtonElement>(null)
  useEffect(() => {
    if (selected) {
      ref.current?.scrollIntoView({ block: "nearest" })
    }
  }, [selected])
  return (
    <button
      ref={ref}
      type="button"
      role="option"
      aria-selected={selected}
      onMouseMove={onSelect}
      onClick={onRun}
      className={cn(
        "group flex w-full items-center gap-3 rounded-md px-3 py-2 text-left outline-none",
        selected ? "bg-accent text-accent-foreground" : "text-foreground",
      )}
    >
      {children}
      <CornerDownLeft
        className={cn(
          "ml-auto size-3.5 shrink-0 text-muted-foreground",
          selected ? "opacity-100" : "opacity-0",
        )}
      />
    </button>
  )
}

function Hint({ keys, children }: { keys: string[]; children: ReactNode }) {
  return (
    <span className="inline-flex items-center gap-1.5">
      <span className="inline-flex gap-1">
        {keys.map((k) => (
          <Keys key={k}>{k}</Keys>
        ))}
      </span>
      {children}
    </span>
  )
}
