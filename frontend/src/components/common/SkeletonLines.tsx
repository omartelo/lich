import { Skeleton } from "@/components/ui/skeleton"
import { cn } from "@/lib/utils"

interface SkeletonLinesProps {
  /** Tailwind width classes, one per line. Ragged on purpose — see below. */
  widths: readonly string[]
  /** Line height class; the default is a line of prose. */
  height?: string
  className?: string
}

// A stack of placeholder bars at ragged widths. The raggedness is the whole
// point: equal bars read as a loading widget, uneven ones read as the text
// that is coming — which is what keeps a screen-sized wait from looking like a
// failure. Each caller brings its own widths, because what is arriving differs
// (prose, file names, code).
export function SkeletonLines({ widths, height = "h-3", className }: SkeletonLinesProps) {
  return (
    <>
      {widths.map((width) => (
        <Skeleton key={width} className={cn(height, width, className)} />
      ))}
    </>
  )
}
