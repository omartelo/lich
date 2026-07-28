import { Skeleton } from "@/components/ui/skeleton"

// Ragged widths, so the body placeholder reads as prose instead of a block.
const BODY_ROWS = ["w-full", "w-11/12", "w-4/5", "w-2/3", "w-5/6", "w-1/2"]

// Looking the pull request up is a gh round-trip, and the screen covers the
// whole app while it runs — a lone "Loading…" in the middle of that much empty
// space reads as a failure. The skeleton holds the header the answer will fill:
// title, actions, status line, tabs, then the body.
export function PullSkeleton() {
  return (
    <div aria-busy>
      <div className="border-b border-border px-6 pt-5">
        <div className="flex items-start gap-4">
          <Skeleton className="h-6 w-80 max-w-full" />
          <div className="ml-auto flex flex-none items-center gap-2">
            <Skeleton className="h-8 w-28" />
            <Skeleton className="size-8" />
            <Skeleton className="h-8 w-24" />
          </div>
        </div>
        <div className="mt-4 flex items-center gap-4">
          <Skeleton className="h-3 w-14" />
          <Skeleton className="h-3 w-56" />
          <Skeleton className="h-3 w-36" />
        </div>
        <div className="mt-5 flex gap-6 pb-3">
          <Skeleton className="h-3 w-16" />
          <Skeleton className="h-3 w-24" />
          <Skeleton className="h-3 w-14" />
        </div>
      </div>
      <div className="flex max-w-3xl flex-col gap-3 px-6 py-6">
        {BODY_ROWS.map((width) => (
          <Skeleton key={width} className={`h-3 ${width}`} />
        ))}
      </div>
    </div>
  )
}
