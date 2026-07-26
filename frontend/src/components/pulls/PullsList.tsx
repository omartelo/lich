import { useMemo, useState } from "react"
import { ChevronDown, GitPullRequestArrow, Search, Terminal } from "lucide-react"
import type { PullRequestSummary } from "@/lib/api-types"
import {
  PULLS_SORTS,
  SORT_LABELS,
  checkVerdict,
  filterCounts,
  filterPullRequests,
  sortPullRequests,
  updatedAgo,
  writePullsSort,
  type CheckVerdict,
  type PullsFilter,
  type PullsSort,
} from "@/lib/pulls/pull-request-list"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { cn } from "@/lib/utils"

// The dot's colour is the only chroma in a row, so it has to mean something: a
// check that failed, one still running, one that passed. A pull request with no
// checks at all shows nothing rather than a grey dot that reads as pending.
const VERDICT_DOT: Record<CheckVerdict, string> = {
  failed: "bg-destructive",
  pending: "bg-muted-foreground",
  passed: "bg-emerald-500",
  none: "",
}

const FILTER_LABELS: ReadonlyArray<[PullsFilter, string]> = [
  ["all", "All"],
  ["ready", "Ready"],
  ["drafts", "Drafts"],
  ["failing", "Failing"],
]

interface PullsListProps {
  list: PullRequestSummary[]
  loading: boolean
  error: string | null
  /** The pull request the detail pane is showing; 0 before one is chosen. */
  selected: number
  onSelect: (number: number) => void
  sort: PullsSort
  onSortChange: (sort: PullsSort) => void
  /** Head branches already checked out somewhere — git refuses a second one. */
  checkedOutBranches: ReadonlySet<string>
}

// PullsList is the Pulls screen's master column: every open pull request of the
// repository, filtered and ranked, with the detail pane following the selection.
export function PullsList({
  list,
  loading,
  error,
  selected,
  onSelect,
  sort,
  onSortChange,
  checkedOutBranches,
}: PullsListProps) {
  const [filter, setFilter] = useState<PullsFilter>("all")
  const [query, setQuery] = useState("")
  const counts = useMemo(() => filterCounts(list), [list])
  const rows = useMemo(
    () => sortPullRequests(filterPullRequests(list, filter, query), sort),
    [list, filter, query, sort],
  )

  const chooseSort = (next: PullsSort) => {
    writePullsSort(next)
    onSortChange(next)
  }

  return (
    <div className="flex w-80 flex-none flex-col border-r border-border">
      <div className="flex flex-col gap-2 px-3 pt-3">
        <div className="relative">
          <Search className="pointer-events-none absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
          <Input
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Filter pull requests"
            aria-label="Filter pull requests"
            spellCheck={false}
            className="h-8 pl-8 text-sm"
          />
        </div>
        {/* The settings screen's SegmentedControl is built for a full-width
            pane; four options with counts wrap in a column this narrow, so this
            is the same idiom at list density — one bordered track, the chosen
            option filled. */}
        <div
          role="radiogroup"
          aria-label="Pull request filter"
          className="flex gap-0.5 rounded-md border border-border bg-muted/40 p-0.5"
        >
          {FILTER_LABELS.map(([value, label]) => {
            const active = value === filter
            return (
              <button
                key={value}
                type="button"
                role="radio"
                aria-checked={active}
                onClick={() => setFilter(value)}
                className={cn(
                  "flex flex-1 items-center justify-center gap-1 rounded-[0.3125rem] px-1 py-1 text-xs transition-colors outline-none focus-visible:ring-2 focus-visible:ring-ring",
                  active
                    ? "bg-accent text-foreground"
                    : "text-muted-foreground hover:text-foreground",
                )}
              >
                {label}
                <span className="tabular-nums opacity-60">{counts[value]}</span>
              </button>
            )
          })}
        </div>
      </div>

      <div className="flex items-center justify-between px-3 py-2 text-xs text-muted-foreground">
        <span className="uppercase tracking-wide">Open</span>
        <DropdownMenu>
          <DropdownMenuTrigger
            render={
              <button
                type="button"
                className="flex items-center gap-1 rounded-md px-1.5 py-0.5 transition-colors hover:bg-accent/50 hover:text-foreground"
              />
            }
          >
            {SORT_LABELS[sort]}
            <ChevronDown className="size-3" />
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            {PULLS_SORTS.map((option) => (
              <DropdownMenuItem key={option} onClick={() => chooseSort(option)}>
                {SORT_LABELS[option]}
              </DropdownMenuItem>
            ))}
          </DropdownMenuContent>
        </DropdownMenu>
      </div>

      <div className="flex flex-1 flex-col gap-0.5 overflow-y-auto px-1.5 pb-3">
        {loading && list.length === 0 ? (
          <ListSkeleton />
        ) : error ? (
          <p className="px-2 py-3 text-xs text-muted-foreground">{error}</p>
        ) : rows.length === 0 ? (
          <p className="px-2 py-3 text-xs text-muted-foreground">
            {list.length === 0 ? "No open pull requests." : "Nothing matches that filter."}
          </p>
        ) : (
          rows.map((pr) => (
            <PullRow
              key={pr.number}
              pr={pr}
              active={pr.number === selected}
              isCheckedOut={checkedOutBranches.has(pr.headRefName)}
              onSelect={() => onSelect(pr.number)}
            />
          ))
        )}
      </div>
    </div>
  )
}

interface PullRowProps {
  pr: PullRequestSummary
  active: boolean
  isCheckedOut: boolean
  onSelect: () => void
}

function PullRow({ pr, active, isCheckedOut, onSelect }: PullRowProps) {
  const verdict = checkVerdict(pr)
  return (
    <button
      type="button"
      aria-current={active ? "true" : undefined}
      onClick={onSelect}
      className={cn(
        "flex w-full gap-2 rounded-md px-2 py-2 text-left transition-colors hover:bg-accent/50",
        active && "bg-accent",
      )}
    >
      <GitPullRequestArrow
        className={cn(
          "mt-0.5 size-3.5 shrink-0",
          pr.isDraft ? "text-amber-500" : "text-muted-foreground",
        )}
      />
      <span className="flex min-w-0 flex-col gap-0.5">
        <span className="line-clamp-2 text-sm leading-snug">
          <span className="font-mono tabular-nums text-muted-foreground">#{pr.number}</span>{" "}
          {pr.title}
        </span>
        <span className="flex items-center gap-2 text-xs text-muted-foreground">
          <span className="truncate">{pr.author}</span>
          <span className="font-mono">{updatedAgo(pr.updatedAt)}</span>
          {verdict !== "none" && (
            <span className={cn("size-1.5 shrink-0 rounded-full", VERDICT_DOT[verdict])} />
          )}
          {pr.isDraft && <span className="text-amber-500">Draft</span>}
          {pr.isCrossRepository && <span>fork</span>}
          {isCheckedOut && (
            <span className="flex items-center gap-1">
              <Terminal className="size-3" />
              checked out
            </span>
          )}
        </span>
      </span>
    </button>
  )
}

// Ragged widths so the placeholder reads as a list of titles, not as a block.
const SKELETON_ROWS = ["w-11/12", "w-4/5", "w-full", "w-3/4", "w-5/6"]

function ListSkeleton() {
  return (
    <div aria-busy className="flex flex-col gap-3 px-2 py-2">
      {SKELETON_ROWS.map((width) => (
        <div key={width} className="flex flex-col gap-1.5">
          <Skeleton className={`h-3 ${width}`} />
          <Skeleton className="h-2.5 w-1/3" />
        </div>
      ))}
    </div>
  )
}
