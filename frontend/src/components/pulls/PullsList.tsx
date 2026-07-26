import { useMemo, useState } from "react"
import {
  ChevronDown,
  GitMerge,
  GitPullRequestArrow,
  GitPullRequestClosed,
  PanelLeftClose,
  PanelLeftOpen,
  Search,
  Terminal,
  type LucideIcon,
} from "lucide-react"
import { IconAction } from "@/components/common/IconAction"
import type { PullRequestSummary } from "@/lib/api-types"
import {
  FILTER_LABELS,
  PULLS_FILTERS,
  PULLS_PAGE_LIMIT,
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
  type PullsQuery,
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

// The column is a navigator, not the review: collapsing it hands the width to
// the pull request. Remembered in localStorage like the Files tab's tree and
// every other UI pref, so the choice survives leaving the screen.
const LIST_HIDDEN_KEY = "lich.pulls.list.hidden"

// The box takes GitHub's own qualifiers, so the syntax is already known to
// anyone who has searched pull requests there. The tooltip is the only place
// that says so — a column this narrow has no room for a legend.
const QUALIFIER_HELP = [
  "Words match the number, title, author and branch.",
  "is:open · is:closed · is:merged · is:all",
  "is:draft · is:ready · is:fork",
  "review:required · review:approved · review:changes-requested",
].join("\n")

interface PullsListProps {
  list: PullRequestSummary[]
  loading: boolean
  error: string | null
  /** The pull request the detail pane is showing; 0 before one is chosen. */
  selected: number
  onSelect: (number: number) => void
  sort: PullsSort
  onSortChange: (sort: PullsSort) => void
  /** The raw filter box, owned by the screen: its `is:` state decides which
   * pull requests gh is asked for, which is a fetch, not a filter. */
  query: string
  onQueryChange: (query: string) => void
  parsed: PullsQuery
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
  query,
  onQueryChange,
  parsed,
  checkedOutBranches,
}: PullsListProps) {
  const [open, setOpen] = useState(() => localStorage.getItem(LIST_HIDDEN_KEY) !== "1")
  const [filter, setFilter] = useState<PullsFilter>("all")
  const counts = useMemo(() => filterCounts(list, parsed), [list, parsed])
  const rows = useMemo(
    () => sortPullRequests(filterPullRequests(list, filter, parsed), sort),
    [list, filter, parsed, sort],
  )

  const chooseSort = (next: PullsSort) => {
    writePullsSort(next)
    onSortChange(next)
  }

  const toggle = () => {
    setOpen((wasOpen) => {
      localStorage.setItem(LIST_HIDDEN_KEY, wasOpen ? "1" : "0")
      return !wasOpen
    })
  }

  // Collapsed, the column keeps a rail: the toggle has to survive its own
  // click, and the seam keeps the pull request from running to the window edge.
  if (!open) {
    return (
      <div className="flex flex-none flex-col items-center border-r border-border px-1.5 pt-3">
        <IconAction label="Show the pull request list" onClick={toggle}>
          <PanelLeftOpen className="size-3.5" />
        </IconAction>
      </div>
    )
  }

  return (
    <div className="flex w-80 flex-none flex-col border-r border-border">
      <div className="flex flex-col gap-2 px-3 pt-3">
        <div className="flex items-center gap-1.5">
          <div className="relative flex-1">
            <Search className="pointer-events-none absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
            <Input
              value={query}
              onChange={(event) => onQueryChange(event.target.value)}
              placeholder="Filter — try is:merged"
              aria-label="Filter pull requests"
              title={QUALIFIER_HELP}
              spellCheck={false}
              className="h-8 pl-8 text-sm"
            />
          </div>
          <IconAction label="Hide the pull request list" onClick={toggle}>
            <PanelLeftClose className="size-3.5" />
          </IconAction>
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
          {PULLS_FILTERS.map((value) => {
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
                {FILTER_LABELS[value]}
                <span className="tabular-nums opacity-60">{counts[value]}</span>
              </button>
            )
          })}
        </div>
      </div>

      <div className="flex items-center justify-between px-3 py-2 text-xs text-muted-foreground">
        <span className="uppercase tracking-wide">
          {parsed.state}
          {list.length >= PULLS_PAGE_LIMIT && (
            <span
              className="ml-1.5 normal-case"
              title={`Only the ${PULLS_PAGE_LIMIT} most recently updated are loaded — filtering searches those.`}
            >
              first {PULLS_PAGE_LIMIT}
            </span>
          )}
        </span>
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
            {list.length === 0
              ? `No ${parsed.state === "all" ? "" : `${parsed.state} `}pull requests.`
              : "Nothing matches that filter."}
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

// State is carried by the glyph's shape, not by a hue: merged and closed rows
// only appear when they were asked for, and DESIGN.md keeps colour for meaning
// the eye has to catch (a failing check, a draft).
const STATE_GLYPH: Record<string, LucideIcon> = {
  MERGED: GitMerge,
  CLOSED: GitPullRequestClosed,
}

// Only a decision worth acting on shows: REVIEW_REQUIRED is the resting state
// of most rows, and labelling every one of them says nothing.
const REVIEW_LABEL: Record<string, { text: string; className: string }> = {
  APPROVED: { text: "approved", className: "text-emerald-500" },
  CHANGES_REQUESTED: { text: "changes requested", className: "text-destructive" },
}

function PullRow({ pr, active, isCheckedOut, onSelect }: PullRowProps) {
  const verdict = checkVerdict(pr)
  const StateGlyph = STATE_GLYPH[pr.state] ?? GitPullRequestArrow
  const review = REVIEW_LABEL[pr.reviewDecision]
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
      <StateGlyph
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
          {review && <span className={review.className}>{review.text}</span>}
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
