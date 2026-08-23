import type { PullRequestSummary } from "@/lib/api-types"
import { agoUnit } from "@/lib/ago"
import { parseEnumPref, readPref, writePref } from "@/lib/prefs"

// The list column's own logic: which rows a quick filter and the search box
// leave standing, and in what order. Kept out of the component because this is
// the part with edge cases (a PR with no checks is not "failing"), and the
// frontend gate cannot render.

/** The quick filters above the list, in the order they are shown. */
export const PULLS_FILTERS = ["all", "ready", "drafts", "failing"] as const
export type PullsFilter = (typeof PULLS_FILTERS)[number]

export const FILTER_LABELS: Record<PullsFilter, string> = {
  all: "All",
  ready: "Ready",
  drafts: "Drafts",
  failing: "Failing",
}

export const PULLS_SORTS = ["updated", "oldest", "failing", "number"] as const
export type PullsSort = (typeof PULLS_SORTS)[number]

export const SORT_LABELS: Record<PullsSort, string> = {
  updated: "Recently updated",
  oldest: "Least recently updated",
  failing: "Failing first",
  number: "Highest number",
}

// How many pull requests one gh call brings back — mirrors prListLimit in
// internal/project/pr.go, and is only ever compared against, never sent. A full
// page means the answer was cut: the column says so, because a query typed over
// a truncated list otherwise reads as the repository's whole answer.
export const PULLS_PAGE_LIMIT = 50

/** What a row's check dot says. "none" is a PR that reports no checks at all. */
export type CheckVerdict = "failed" | "pending" | "passed" | "none"

// A single failure outranks anything still running: red is what the eye should
// find. Nothing reported is its own verdict — silence is not a pass.
export function checkVerdict(pr: PullRequestSummary): CheckVerdict {
  if (pr.checks.total === 0) {
    return "none"
  }
  if (pr.checks.failed > 0) {
    return "failed"
  }
  return pr.checks.pending > 0 ? "pending" : "passed"
}

/** Which pull requests gh is asked for; anything else is filtered in the page. */
const PULLS_STATES = ["open", "closed", "merged", "all"] as const
export type PullsState = (typeof PULLS_STATES)[number]

const REVIEWS = ["required", "approved", "changes-requested"] as const
type ReviewFilter = (typeof REVIEWS)[number]

// gh's reviewDecision, in the words the query uses. "" — a repository that
// requires no review — matches nothing, since asking for a review state means
// asking for one that exists.
const REVIEW_OF: Record<string, ReviewFilter> = {
  REVIEW_REQUIRED: "required",
  APPROVED: "approved",
  CHANGES_REQUESTED: "changes-requested",
}

export interface PullsQuery {
  state: PullsState
  review: ReviewFilter | null
  /** true for `is:draft`, false for `is:ready`, null when the query is silent. */
  draft: boolean | null
  /** true for `is:fork`, null when the query is silent. Deliberately not a
   * boolean: GitHub spells no "not a fork" qualifier, so nothing can ask for the
   * complement, and a `false` here would be a filter no query can reach. */
  fork: true | null
  /** Everything that was not a qualifier, matched against the row's text. */
  text: string
}

// parsePullsQuery reads GitHub's own qualifier syntax out of the filter box —
// `is:merged`, `review:approved`, `is:draft` — and leaves the rest as the text
// to match. A qualifier this build does not know stays text on purpose: a typo
// then narrows the list to nothing visible rather than being silently dropped,
// which reads as a filter that does not work.
export function parsePullsQuery(raw: string): PullsQuery {
  const query: PullsQuery = { state: "open", review: null, draft: null, fork: null, text: "" }
  const words: string[] = []
  for (const word of raw.split(/\s+/)) {
    const value = word.slice(word.indexOf(":") + 1).toLowerCase()
    const known = word.toLowerCase().startsWith("is:")
      ? applyIs(query, value)
      : word.toLowerCase().startsWith("review:")
        ? applyReview(query, value)
        : false
    if (!known && word !== "") {
      words.push(word)
    }
  }
  query.text = words.join(" ")
  return query
}

function applyIs(query: PullsQuery, value: string): boolean {
  if ((PULLS_STATES as readonly string[]).includes(value)) {
    query.state = value as PullsState
    return true
  }
  switch (value) {
    case "draft":
      query.draft = true
      return true
    case "ready":
      query.draft = false
      return true
    case "fork":
      query.fork = true
      return true
    default:
      return false
  }
}

function applyReview(query: PullsQuery, value: string): boolean {
  const review = REVIEWS.find((r) => r === value || r.replace("-", "_") === value)
  if (!review) {
    return false
  }
  query.review = review
  return true
}

function matchesQualifiers(pr: PullRequestSummary, query: PullsQuery): boolean {
  if (query.draft !== null && pr.isDraft !== query.draft) {
    return false
  }
  if (query.fork !== null && !pr.isCrossRepository) {
    return false
  }
  if (query.review !== null && REVIEW_OF[pr.reviewDecision] !== query.review) {
    return false
  }
  return true
}

function matchesFilter(pr: PullRequestSummary, filter: PullsFilter): boolean {
  switch (filter) {
    case "ready":
      return !pr.isDraft
    case "drafts":
      return pr.isDraft
    case "failing":
      return checkVerdict(pr) === "failed"
    default:
      return true
  }
}

// The number matches with or without its "#", because that is how a PR is
// named out loud and pasted from anywhere else.
function matchesText(pr: PullRequestSummary, text: string): boolean {
  const needle = text.trim().toLowerCase()
  if (needle === "") {
    return true
  }
  const haystack = [`#${pr.number}`, String(pr.number), pr.title, pr.author, pr.headRefName]
  return haystack.some((field) => field.toLowerCase().includes(needle))
}

export function filterPullRequests(
  list: PullRequestSummary[],
  filter: PullsFilter,
  query: PullsQuery,
): PullRequestSummary[] {
  return list.filter(
    (pr) =>
      matchesFilter(pr, filter) && matchesQualifiers(pr, query) && matchesText(pr, query.text),
  )
}

// How many rows each quick filter would leave, for the counts on the buttons.
// Counted over what the query already left standing, not over the whole list:
// the buttons sit under the filter box, so a count that ignored it would
// contradict the rows right below them.
export function filterCounts(
  list: PullRequestSummary[],
  query: PullsQuery,
): Record<PullsFilter, number> {
  const counts = { all: 0, ready: 0, drafts: 0, failing: 0 }
  for (const pr of list) {
    if (!matchesQualifiers(pr, query) || !matchesText(pr, query.text)) {
      continue
    }
    for (const filter of PULLS_FILTERS) {
      if (matchesFilter(pr, filter)) {
        counts[filter]++
      }
    }
  }
  return counts
}

// Worst first, so "Failing first" reads top-down as what needs attention.
const VERDICT_RANK: Record<CheckVerdict, number> = {
  failed: 0,
  pending: 1,
  none: 2,
  passed: 3,
}

// gh's timestamps are ISO-8601 in UTC, so they sort lexicographically — no Date
// parsing, and a malformed one sinks to the end instead of becoming NaN.
function byUpdatedDesc(a: PullRequestSummary, b: PullRequestSummary): number {
  return b.updatedAt.localeCompare(a.updatedAt)
}

export function sortPullRequests(
  list: PullRequestSummary[],
  sort: PullsSort,
): PullRequestSummary[] {
  const sorted = [...list]
  switch (sort) {
    case "oldest":
      return sorted.sort((a, b) => a.updatedAt.localeCompare(b.updatedAt))
    case "number":
      return sorted.sort((a, b) => b.number - a.number)
    case "failing":
      return sorted.sort(
        (a, b) =>
          VERDICT_RANK[checkVerdict(a)] - VERDICT_RANK[checkVerdict(b)] || byUpdatedDesc(a, b),
      )
    default:
      return sorted.sort(byUpdatedDesc)
  }
}

// updatedAgo renders how long ago a pull request last moved, at the coarsest
// unit that still says something — a row has space for "3h", not for a date and
// a time. now is injectable so a test does not depend on the clock.
//
// The scale itself is shared with the palette's closed sessions (lib/ago.ts):
// what differs here is only the input, gh's ISO timestamp.
export function updatedAgo(updatedAt: string, now: number = Date.now()): string {
  const at = Date.parse(updatedAt)
  return Number.isNaN(at) ? "" : agoUnit(now - at)
}

// Which order the column is in is a UI preference, so it lives in the page's
// localStorage beside the panel widths — not in the workspace database.
const SORT_KEY = "lich.pulls.sort"

/** Reads a stored sort, falling back for anything this build does not know. */
export function parsePullsSort(raw: string | null): PullsSort {
  return parseEnumPref(raw, PULLS_SORTS, "updated")
}

export function readPullsSort(): PullsSort {
  return parsePullsSort(readPref(SORT_KEY))
}

export function writePullsSort(sort: PullsSort): void {
  writePref(SORT_KEY, sort)
}
