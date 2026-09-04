import { useState } from "react"
import type { PullRequestDetail } from "@/lib/api-types"
import { foldConflicts, splitConflictPath } from "@/lib/pulls/conflict-files"
import { conflictsWithBase } from "@/lib/pulls/merge-gate"
import { usePullRequestConflicts } from "@/lib/pulls/use-pull-request-conflicts"

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
    detail.url,
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

  const { shown, hidden } = foldConflicts(files, all)
  return (
    <>
      <div className="mt-1.5 flex flex-wrap items-baseline gap-x-3 gap-y-1 text-xs">
        <span className="text-muted-foreground">
          {files.length} conflicting {files.length === 1 ? "file" : "files"}
        </span>
        {shown.map((file) => (
          <ConflictPath key={file} file={file} />
        ))}
        <MoreButton hidden={hidden} all={all} onToggle={() => setAll(!all)} />
      </div>
      {all && <ConflictList files={files} />}
    </>
  )
}

// The rest of the list, and the way back. Nothing is drawn while every path is
// already named: a button offering to unfold what is open is a dead click.
function MoreButton({
  hidden,
  all,
  onToggle,
}: {
  hidden: number
  all: boolean
  onToggle: () => void
}) {
  if (hidden === 0) {
    return null
  }
  return (
    <button
      type="button"
      onClick={onToggle}
      className="text-muted-foreground underline underline-offset-2 transition-colors hover:text-foreground"
    >
      {all ? "Show fewer" : `+${hidden} more`}
    </button>
  )
}

// One per line once the list is open, and bounded: a merge gone this wrong can
// name dozens of files, and the header is not the place to scroll for them — the
// count above is the reading that matters by then.
function ConflictList({ files }: { files: string[] }) {
  return (
    <div className="mt-1.5 flex max-h-20 flex-col gap-0.5 overflow-y-auto text-xs">
      {files.map((file) => (
        <ConflictPath key={file} file={file} />
      ))}
    </div>
  )
}

// One path, with its directory dimmed (conflict-files).
function ConflictPath({ file }: { file: string }) {
  const { dir, name } = splitConflictPath(file)
  return (
    <span className="font-mono text-destructive">
      {dir && <span className="opacity-70">{dir}</span>}
      {name}
    </span>
  )
}

// The row's other three states — reading, failed, nothing left — all say one
// sentence in the same place the paths would have gone.
function Row({ children }: { children: React.ReactNode }) {
  return <div className="mt-1.5 text-xs text-muted-foreground">{children}</div>
}
