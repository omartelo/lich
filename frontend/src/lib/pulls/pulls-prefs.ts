import { parseEnumPref, readPref, writePref } from "@/lib/prefs"
import { PULLS_FILTERS, PULLS_SORTS, type PullsFilter, type PullsSort } from "./pull-request-list"

// Everything the pull request screen remembers about how it was left. A review
// is not a page view: the user walks off to another project mid-diff and comes
// back expecting the screen they had, and a filter box that has emptied itself
// is a review typed twice.
//
// UI preferences, so the page's localStorage rather than the workspace database
// (see the root CLAUDE.md). Parsing is the half kept pure and tested; reading
// and writing the key stays a two-line wrapper, the split prefs.ts describes.
//
// One rule decides the scope of each of these: anything that *narrows the list*
// is about a repository's content and is keyed per project — a box left on
// `is:merged` would otherwise make the next project's list look empty of open
// pull requests. How the column is read (the sort) and where a pull request
// opens (the tab) are habits of this user, and are global.
const SORT_KEY = "lich.pulls.sort"
const TAB_KEY = "lich.pulls.tab"
const FILTER_PREFIX = "lich.pulls.filter."
const QUERY_PREFIX = "lich.pulls.query."
const LAST_PULL_PREFIX = "lich.pulls.last."
const ACTIVE_FILE_PREFIX = "lich.pulls.file."

/** The tabs of one pull request, in the order the header shows them. */
export const PULLS_TABS = ["overview", "commits", "files", "conversation", "checks"] as const
export type PullsTab = (typeof PULLS_TABS)[number]

export function readPullsSort(): PullsSort {
  return parseEnumPref(readPref(SORT_KEY), PULLS_SORTS, "updated")
}

export function writePullsSort(sort: PullsSort): void {
  writePref(SORT_KEY, sort)
}

export function readPullsFilter(projectId: string): PullsFilter {
  return parseEnumPref(scoped(FILTER_PREFIX, projectId), PULLS_FILTERS, "all")
}

export function writePullsFilter(projectId: string, filter: PullsFilter): void {
  if (projectId) {
    writePref(`${FILTER_PREFIX}${projectId}`, filter)
  }
}

export function readPullsTab(): PullsTab {
  return parseEnumPref(readPref(TAB_KEY), PULLS_TABS, "overview")
}

export function writePullsTab(tab: PullsTab): void {
  writePref(TAB_KEY, tab)
}

/** The filter box, verbatim — it is free text, so anything stored is valid. */
export function readPullsQuery(projectId: string): string {
  return scoped(QUERY_PREFIX, projectId) ?? ""
}

export function writePullsQuery(projectId: string, query: string): void {
  if (projectId) {
    writePref(`${QUERY_PREFIX}${projectId}`, query)
  }
}

// Every scoped read goes through here, so a screen whose id is not resolved
// yet — a project comes straight from the route, a pull request from a lookup
// still in flight — reads the default rather than whatever sits under a key
// ending in nothing.
function scoped(prefix: string, id: string): string | null {
  return id ? readPref(`${prefix}${id}`) : null
}

// Which pull request the list column had selected. Every way back into the
// screen navigates to the bare list route, so without this a return lands on
// the checkout's own pull request rather than the one being read.
//
// Nothing forgets it on a failed lookup. A pull request that no longer resolves
// is rare, and lich already says so in as many words ("GitHub has no pull
// request with that number") with the list beside it to pick from — while every
// *transient* failure looks the same from here, so forgetting on one would let
// an offline launch quietly erase the selection of every project it touched.
export function readLastPull(projectId: string): number {
  // 0 for a project with none — which is also what a stored value this build
  // cannot read as a positive number means.
  const number = Number(scoped(LAST_PULL_PREFIX, projectId))
  return Number.isInteger(number) && number > 0 ? number : 0
}

export function writeLastPull(projectId: string, number: number): void {
  if (projectId && number > 0) {
    writePref(`${LAST_PULL_PREFIX}${projectId}`, number)
  }
}

// Which file of a pull request's diff the changed-files tree has marked: the
// one the reviewer jumped to. Keyed by the pull request rather than the project
// — it addresses that pull request's content, and the same reviewer is reading
// a different file in each of them.
//
// Free text, like the filter box: a path is whatever the diff called it, so
// anything stored is readable. A path the diff no longer carries marks no row,
// which is what a file dropped by a force-push should look like.
export function readActiveFile(pullRequest: string): string {
  return scoped(ACTIVE_FILE_PREFIX, pullRequest) ?? ""
}

export function writeActiveFile(pullRequest: string, path: string): void {
  if (pullRequest && path) {
    writePref(`${ACTIVE_FILE_PREFIX}${pullRequest}`, path)
  }
}
