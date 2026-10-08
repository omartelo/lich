import { useEffect, useMemo, useState, type KeyboardEvent } from "react"
import { Notice } from "@/components/common/Notice"
import { FileIcon } from "@/components/FileIcon"
import type { SearchResult } from "@/lib/api-types"
import {
  groupMatches,
  highlightSegments,
  searchFootnote,
  searchSummary,
  type SearchGroup,
} from "@/lib/git/file-search"
import { splitPath } from "@/lib/git/lang-badge"
import { ProjectService } from "@/lib/rpc"
import { useRemoteResource } from "@/lib/use-remote-resource"

// How long typing has to pause before the checkout is read: one search reads
// every file the tree lists, and a word typed at speed is not five searches.
const SETTLE_MS = 250

// What the list holds before its first answer, and after a failed search. A
// module-level constant because useRemoteResource compares it by identity.
const NO_SEARCH: SearchResult = { matches: [], cut: false, tooLarge: 0 }

/** Finds the hit rows, so the box above can hand focus down into the list. */
export const HIT_SELECTOR = "[data-search-hit]"

interface SearchResultsProps {
  path: string
  query: string
  /** The tree's own staleness key: whatever re-reads the tree re-runs the
   * search, so a file the agent just wrote is searched without a keystroke. */
  refreshKey: string
  /** What the tree owes the reader about its listing (treeFootnote), which the
   * search inherits: it reads that same listing. */
  listingNote: string
  onOpen: (rel: string, line: number) => void
}

// SearchResults is the Code tab in text mode: the lines holding the query,
// one group per file, each opening the preview on its line.
export function SearchResults({
  path,
  query,
  refreshKey,
  listingNote,
  onOpen,
}: SearchResultsProps) {
  const settled = useSettled(query.trim(), SETTLE_MS)
  // Reset on the checkout only: a new query keeps the last answer on screen
  // until its own lands, so typing does not flash the list empty on every
  // pause, but another checkout's hits never stand in for this one's.
  const { data, loading, error } = useRemoteResource(
    settled && refreshKey ? `${refreshKey} ${settled}` : "",
    () => ProjectService.Search(path, settled),
    { empty: NO_SEARCH, resetOn: path },
  )
  const groups = useMemo(() => groupMatches(data.matches), [data.matches])

  if (settled === "") {
    return <Notice>Search the text of every file in this checkout</Notice>
  }
  if (error !== null) {
    return <Notice>{error}</Notice>
  }
  if (loading && data === NO_SEARCH) {
    return <Notice>Searching…</Notice>
  }
  const footnote = [searchFootnote(data), listingNote].filter(Boolean).join(" ")
  return (
    <>
      {groups.length === 0 ? (
        <Notice className="flex-1">No file contains “{settled}”</Notice>
      ) : (
        <div className="min-h-0 flex-1 overflow-auto" data-search-list="">
          <p className="px-3 pt-2 pb-1 text-2xs uppercase tracking-wide text-muted-foreground">
            {searchSummary(data, groups.length)}
          </p>
          <div className="pb-1 font-mono text-xs">
            {groups.map((group) => (
              <HitGroup key={group.path} group={group} query={query.trim()} onOpen={onOpen} />
            ))}
          </div>
        </div>
      )}
      {footnote !== "" && (
        <Notice className="shrink-0 border-t border-border py-2">{footnote}</Notice>
      )}
    </>
  )
}

interface HitGroupProps {
  group: SearchGroup
  query: string
  onOpen: (rel: string, line: number) => void
}

function HitGroup({ group, query, onOpen }: HitGroupProps) {
  const { dir, base } = splitPath(group.path)
  return (
    <>
      <div className="flex items-center gap-1.5 px-2 pt-1.5 pb-0.5" title={group.path}>
        <FileIcon path={group.path} />
        <span className="min-w-0 flex-1 truncate">
          {dir && <span className="text-muted-foreground">{dir}/</span>}
          {base}
        </span>
        <span className="shrink-0 tabular-nums text-muted-foreground">{group.hits.length}</span>
      </div>
      {group.hits.map((hit) => (
        <button
          key={hit.line}
          type="button"
          data-search-hit=""
          onClick={() => onOpen(hit.path, hit.line)}
          onKeyDown={moveBetweenHits}
          className="flex w-full items-baseline gap-2 rounded-md py-0.5 pr-2 pl-6 text-left transition-colors outline-none hover:bg-accent/50 focus-visible:bg-accent"
        >
          <span className="w-[4ch] shrink-0 text-right tabular-nums text-muted-foreground">
            {hit.line}
          </span>
          <span className="min-w-0 flex-1 truncate">
            {highlightSegments(hit.text, query).map((segment, i) =>
              segment.hit ? (
                // biome-ignore lint/suspicious/noArrayIndexKey: segments are positional and never reorder
                <mark key={i} className="rounded-sm bg-tone-wait/30 text-foreground">
                  {segment.text}
                </mark>
              ) : (
                // biome-ignore lint/suspicious/noArrayIndexKey: segments are positional and never reorder
                <span key={i}>{segment.text}</span>
              ),
            )}
          </span>
        </button>
      ))}
    </>
  )
}

// moveBetweenHits walks focus through the hit rows with the arrow keys; Enter
// is the focused button's own click.
function moveBetweenHits(event: KeyboardEvent<HTMLButtonElement>) {
  if (event.key !== "ArrowDown" && event.key !== "ArrowUp") {
    return
  }
  const list = event.currentTarget.closest("[data-search-list]")
  const hits = [...(list?.querySelectorAll<HTMLElement>(HIT_SELECTOR) ?? [])]
  const at = hits.indexOf(event.currentTarget)
  const next = hits[at + (event.key === "ArrowDown" ? 1 : -1)]
  if (next) {
    event.preventDefault()
    next.focus()
  }
}

// useSettled answers value once it has held still for ms; an empty value
// answers at once, since clearing the box is not typing to wait out.
function useSettled(value: string, ms: number): string {
  const [settled, setSettled] = useState(value)
  useEffect(() => {
    if (value === "") {
      setSettled("")
      return
    }
    const timer = window.setTimeout(() => setSettled(value), ms)
    return () => window.clearTimeout(timer)
  }, [value, ms])
  return settled
}
