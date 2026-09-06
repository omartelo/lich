import { Paperclip } from "lucide-react"
import { dropHintDetail } from "@/lib/terminal/drop-hint"

interface TerminalDropHintProps {
  label: string
  confined: boolean
}

// TerminalDropHint covers a terminal while a file drag is over it and says
// which session the file lands in. The fill is the "where": one pane of a
// split lights, its neighbour does not. The name is set at reading size
// because the eye is on the cursor, mid-pane, not on a footer. What the drop
// does is the muted line under it, dropped on a pane too short to hold both.
export function TerminalDropHint({ label, confined }: TerminalDropHintProps) {
  return (
    <div className="pointer-events-none absolute inset-0 z-10 grid place-items-center bg-accent/45 [container-type:size]">
      <div className="grid justify-items-center gap-1.5 px-4 text-center">
        <Paperclip className="size-5 text-foreground" />
        <span className="text-sm text-foreground">
          Attach to <span className="font-semibold">{label}</span>
        </span>
        <span className="text-xs text-muted-foreground [@container(max-height:120px)]:hidden">
          {dropHintDetail(confined)}
        </span>
      </div>
    </div>
  )
}
