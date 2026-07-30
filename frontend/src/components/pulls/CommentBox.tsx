import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

interface CommentBoxProps {
  value: string
  onChange: (value: string) => void
  onSubmit: () => void
  /** Absent leaves no way out — for a box that is always open (the review
   * summary), where cancelling means clearing the text. */
  onCancel?: () => void
  submitLabel: string
  busy?: boolean
  placeholder?: string
  autoFocus?: boolean
  rows?: number
  className?: string
}

// The one place review text is typed: a line comment, a reply, the summary above
// a review. It holds no draft of its own — the caller owns the text, because the
// editor under this box is rebuilt by CodeMirror whenever the diff refetches and
// a box that kept its own state would lose it there.
//
// Cmd/Ctrl+Enter sends and Escape backs out, so a comment written mid-review
// never needs the mouse.
export function CommentBox({
  value,
  onChange,
  onSubmit,
  onCancel,
  submitLabel,
  busy = false,
  placeholder,
  autoFocus = false,
  rows = 3,
  className,
}: CommentBoxProps) {
  const empty = value.trim() === ""
  return (
    <div className={cn("flex flex-col gap-2", className)}>
      <textarea
        value={value}
        onChange={(e) => onChange(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter" && (e.metaKey || e.ctrlKey) && !empty) {
            e.preventDefault()
            onSubmit()
          }
          if (e.key === "Escape" && onCancel) {
            e.preventDefault()
            onCancel()
          }
        }}
        rows={rows}
        placeholder={placeholder}
        // biome-ignore lint/a11y/noAutofocus: the box only exists because the reviewer just asked for it — typing is the next thing they do.
        autoFocus={autoFocus}
        className="w-full resize-y rounded-md border border-input bg-transparent px-2.5 py-1.5 font-sans text-sm shadow-xs outline-none transition-[color,box-shadow] placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 dark:bg-input/30"
      />
      <div className="flex items-center gap-2">
        <Button size="sm" onClick={onSubmit} disabled={busy || empty}>
          {busy ? "Sending…" : submitLabel}
        </Button>
        {onCancel && (
          <Button size="sm" variant="ghost" onClick={onCancel} disabled={busy}>
            Cancel
          </Button>
        )}
      </div>
    </div>
  )
}
