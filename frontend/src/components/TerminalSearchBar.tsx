import { ArrowDown, ArrowUp, X } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"

/** The match position xterm's search addon reports: the active match index
 * (0-based, -1 when none) and the total count. */
export interface SearchResults {
  index: number
  count: number
}

interface TerminalSearchBarProps {
  query: string
  /** Null before the addon has reported anything for the current query. */
  results: SearchResults | null
  /** The query changed: search incrementally, keeping the current match. */
  onQueryChange: (query: string) => void
  onFind: (direction: "next" | "prev") => void
  onClose: () => void
}

// The in-terminal find box (Ctrl+F), floating over the top right of the
// terminal it searches. It owns nothing: the query, the match count and every
// jump belong to the terminal, which is where the xterm addon lives.
export function TerminalSearchBar({
  query,
  results,
  onQueryChange,
  onFind,
  onClose,
}: TerminalSearchBarProps) {
  return (
    <div className="absolute right-3 top-3 z-20 flex items-center gap-1 rounded-md border bg-popover p-1 text-popover-foreground shadow-lg">
      <Input
        autoFocus
        value={query}
        placeholder="Find"
        aria-label="Search terminal"
        className="h-7 w-44 border-0 shadow-none focus-visible:ring-0"
        onChange={(event) => onQueryChange(event.target.value)}
        onKeyDown={(event) => {
          if (event.key === "Enter") {
            event.preventDefault()
            onFind(event.shiftKey ? "prev" : "next")
          } else if (event.key === "Escape") {
            event.preventDefault()
            onClose()
          }
        }}
      />
      <span className="min-w-10 px-1 text-center text-xs tabular-nums text-muted-foreground">
        {results && results.count > 0
          ? `${results.index + 1}/${results.count}`
          : query
            ? "0/0"
            : ""}
      </span>
      <Button
        size="icon-xs"
        variant="ghost"
        aria-label="Previous match"
        onClick={() => onFind("prev")}
      >
        <ArrowUp className="h-3.5 w-3.5" />
      </Button>
      <Button size="icon-xs" variant="ghost" aria-label="Next match" onClick={() => onFind("next")}>
        <ArrowDown className="h-3.5 w-3.5" />
      </Button>
      <Button size="icon-xs" variant="ghost" aria-label="Close search" onClick={onClose}>
        <X className="h-3.5 w-3.5" />
      </Button>
    </div>
  )
}
