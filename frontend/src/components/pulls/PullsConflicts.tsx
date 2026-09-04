import { useState } from "react"
import type { PullRequestDetail } from "@/lib/api-types"
import { conflictsWithBase } from "@/lib/pulls/merge-gate"
import { usePullRequestConflicts } from "@/lib/pulls/use-pull-request-conflicts"

// How many paths the row names before it folds the rest away. Six fills one
// line of the header at an ordinary window width; past that the row stops
// answering faster and starts pushing the tabs down the screen.
const NAMED = 6

// PullsConflicts names the files a conflicting pull request collides with its
// base on, under the status line that says it conflicts at all. GitHub's own
// answer is the one word the chip above already carries — this is the half that
// sent the reader to github.com or into a session to find out which files it
// meant.
//
// Nothing is drawn for a pull request that merges: the row exists only while
// something is in the way, like every other reading on that line.
export function PullsConflicts({ path, detail }: { path: string; detail: PullRequestDetail }) {
  const [all, setAll] = useState(false)
  const conflicting = detail.state === "OPEN" && conflictsWithBase(detail)
  const { files, loading, error } = usePullRequestConflicts(
    path,
    detail.number,
    detail.baseRefName,
    conflicting,
  )

  if (!conflicting) {
    return null
  }
  if (loading) {
    return <Row>Working out which files conflict…</Row>
  }
  if (error) {
    return <Row>Couldn’t work out which files conflict: {error}</Row>
  }
  // GitHub recomputes mergeability on its own clock, so a resolution that has
  // just landed reads as a conflict up here and as a clean merge down there.
  // Saying which of the two is the fresh one beats a list that silently empties.
  if (files.length === 0) {
    return <Row>No file conflicts here now — GitHub may not have caught up.</Row>
  }

  const hidden = files.length - NAMED
  return (
    <>
      <div className="mt-1.5 flex flex-wrap items-baseline gap-x-3 gap-y-1 text-xs">
        <span className="text-muted-foreground">
          {files.length} conflicting {files.length === 1 ? "file" : "files"}
        </span>
        {!all && files.slice(0, NAMED).map((file) => <ConflictPath key={file} file={file} />)}
        {hidden > 0 && (
          <button
            type="button"
            onClick={() => setAll(!all)}
            className="text-muted-foreground underline underline-offset-2 transition-colors hover:text-foreground"
          >
            {all ? "Show fewer" : `+${hidden} more`}
          </button>
        )}
      </div>
      {/* One per line once it is open, and bounded: a merge gone this wrong can
          name dozens of files, and the header is not the place to scroll for
          them — the count above is the reading that matters by then. */}
      {all && (
        <div className="mt-1.5 flex max-h-20 flex-col gap-0.5 overflow-y-auto text-xs">
          {files.map((file) => (
            <ConflictPath key={file} file={file} />
          ))}
        </div>
      )}
    </>
  )
}

// One path, with its directory dimmed. A monorepo's conflicting files share
// most of their path, and the half that tells them apart is the last segment.
function ConflictPath({ file }: { file: string }) {
  const cut = file.lastIndexOf("/") + 1
  return (
    <span className="font-mono text-destructive">
      {cut > 0 && <span className="opacity-70">{file.slice(0, cut)}</span>}
      {file.slice(cut)}
    </span>
  )
}

// The row's other three states — reading, failed, nothing left — all say one
// sentence in the same place the paths would have gone.
function Row({ children }: { children: React.ReactNode }) {
  return <div className="mt-1.5 text-xs text-muted-foreground">{children}</div>
}
