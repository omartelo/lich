import type { SearchMatch, SearchResult } from "@/lib/api-types"
import { t } from "@/lib/i18n/i18n"

/** One file's hits, in the order the backend found them. */
export interface SearchGroup {
  path: string
  hits: SearchMatch[]
}

// groupMatches folds the backend's flat list, already file by file, into one
// group per file for the list's file rows.
export function groupMatches(matches: readonly SearchMatch[]): SearchGroup[] {
  const groups: SearchGroup[] = []
  for (const match of matches) {
    const last = groups[groups.length - 1]
    if (last?.path === match.path) {
      last.hits.push(match)
    } else {
      groups.push({ path: match.path, hits: [match] })
    }
  }
  return groups
}

export interface Segment {
  text: string
  hit: boolean
}

// highlightSegments splits a hit's line into the runs that match query and the
// runs between them, case-insensitively and as a literal, the way the backend
// matched. The query is the one typed now rather than the one the line came
// back for, so for the moment a new search is in flight a line may light up
// nothing; it still reads.
export function highlightSegments(text: string, query: string): Segment[] {
  const needle = query.toLowerCase()
  if (needle === "") {
    return [{ text, hit: false }]
  }
  const hay = text.toLowerCase()
  // Lowercasing can change a string's length ("İ"), and then offsets into hay
  // no longer land on text: the line is shown plain rather than cut wrong.
  if (hay.length !== text.length) {
    return [{ text, hit: false }]
  }
  const segments: Segment[] = []
  let from = 0
  for (let at = hay.indexOf(needle); at >= 0; at = hay.indexOf(needle, from)) {
    if (at > from) {
      segments.push({ text: text.slice(from, at), hit: false })
    }
    segments.push({ text: text.slice(at, at + needle.length), hit: true })
    from = at + needle.length
  }
  if (from < text.length) {
    segments.push({ text: text.slice(from), hit: false })
  }
  return segments
}

// searchSummary is the label over the list. A cut answer counts lines it
// stopped at, not files: the files past the cap were never read.
export function searchSummary(result: SearchResult, groups: number): string {
  const lines = result.matches.length
  if (result.cut) {
    return t("git.fileSearch.cutMatches", { count: lines })
  }
  return t("git.fileSearch.summary", {
    matches: t("git.fileSearch.matches", { count: lines }),
    files: t("common.count.file", { count: groups }),
  })
}

// searchFootnote is what the answer owes the reader about what it did not
// read: the lines past the cap, and the files the tree shows but the search
// skipped for size. Empty when it read everything it was given.
export function searchFootnote(result: SearchResult): string {
  const parts: string[] = []
  if (result.cut) {
    parts.push(t("git.fileSearch.cutNote", { count: result.matches.length }))
  }
  if (result.tooLarge > 0) {
    parts.push(t("git.fileSearch.tooLarge", { count: result.tooLarge }))
  }
  return parts.join(" ")
}
