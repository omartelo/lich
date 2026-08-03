import type { ReviewThread } from "@/lib/api-types"
import { gutterNumber } from "@/lib/git/diff"
import { threadHunk } from "@/lib/pulls/thread-hunk"
import { cn } from "@/lib/utils"

interface ThreadHunkProps {
  /** GitHub's own hunk, as it travels with the comment. */
  diffHunk: string
  thread: ReviewThread
}

// The lines a review thread was filed on, coloured and numbered the way the
// diff viewer colours them. The diff renders them itself where the thread is
// anchored; this is for the Conversation tab, where the file is not on screen.
export function ThreadHunk({ diffHunk, thread }: ThreadHunkProps) {
  const lines = threadHunk(diffHunk, thread)
  if (lines.length === 0) {
    return null
  }

  return (
    <div className="overflow-x-auto rounded-md bg-background py-1 font-mono text-xs leading-relaxed">
      {/* w-max so a line wider than the card carries its tint the whole scrolled
          width instead of stopping at the viewport edge. */}
      <div className="w-max min-w-full">
        {lines.map((line) => (
          <div
            key={`${line.oldLine}:${line.newLine}`}
            className={cn(
              "flex",
              line.kind === "add" && "diff-add",
              line.kind === "del" && "diff-del",
            )}
          >
            <span
              className={cn(
                "w-12 shrink-0 select-none pr-3 text-right tabular-nums text-muted-foreground/60",
                line.kind === "add" && "diff-gutter-add",
                line.kind === "del" && "diff-gutter-del",
              )}
            >
              {gutterNumber(line)}
            </span>
            <span className="whitespace-pre pr-3">{line.text}</span>
          </div>
        ))}
      </div>
    </div>
  )
}
