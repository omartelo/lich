import { useState } from "react"
import { Ellipsis } from "lucide-react"
import { Notice } from "@/components/common/Notice"
import type { PullRequestCommit } from "@/lib/api-types"
import { cn } from "@/lib/utils"

// PullsCommits is the "Commits" tab of the Pulls screen: every commit the pull
// request would land, oldest first as gh lists them — the story of the branch,
// which the diff never tells. A row is its subject line; the message body waits
// behind the toggle, so a branch of fifteen commits stays a list you can scan.
export function PullsCommits({ commits }: { commits: PullRequestCommit[] | null }) {
  if (!commits || commits.length === 0) {
    return <Notice className="px-6 py-5 text-sm">No commits.</Notice>
  }
  return (
    <div className="flex flex-col py-1">
      {commits.map((commit) => (
        <CommitRow key={commit.oid} commit={commit} />
      ))}
    </div>
  )
}

function CommitRow({ commit }: { commit: PullRequestCommit }) {
  const [open, setOpen] = useState(false)
  const body = commit.body.trim()
  return (
    <div className="px-6 py-2">
      <div className="flex items-center gap-2">
        <span className="min-w-0 flex-1 truncate text-sm font-medium" title={commit.headline}>
          {commit.headline}
        </span>
        {body && (
          <button
            type="button"
            onClick={() => setOpen(!open)}
            aria-expanded={open}
            aria-label={open ? "Hide commit description" : "Show commit description"}
            className={cn(
              "inline-flex shrink-0 items-center rounded-sm px-1 py-0.5 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground",
              open && "bg-accent text-accent-foreground",
            )}
          >
            <Ellipsis className="size-3.5" />
          </button>
        )}
        <span className="shrink-0 font-mono text-xs text-muted-foreground" title={commit.oid}>
          {commit.oid.slice(0, 7)}
        </span>
      </div>
      <p className="text-xs text-muted-foreground">{commitMeta(commit)}</p>
      {open && (
        <p className="mt-2 whitespace-pre-wrap text-xs leading-relaxed text-muted-foreground">
          {body}
        </p>
      )}
    </div>
  )
}

// Who landed it and when, in git's own phrasing. Either half can be missing — a
// merge commit from the web flow carries no author — so the line is built from
// what is there instead of printing an empty "committed on".
function commitMeta(commit: PullRequestCommit): string {
  const at = new Date(commit.date)
  const date = Number.isNaN(at.getTime()) ? "" : at.toLocaleDateString()
  if (commit.author && date) {
    return `${commit.author} committed ${date}`
  }
  return commit.author || (date && `Committed ${date}`) || ""
}
