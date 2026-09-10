import { useState } from "react"
import { ChevronRight } from "lucide-react"
import { Notice } from "@/components/common/Notice"
import type { PullRequestCommit } from "@/lib/api-types"
import { commitAuthorNotice, commitMetaLine } from "@/lib/pulls/commit-authors"
import { cn } from "@/lib/utils"

// PullsCommits is the "Commits" tab of the Pulls screen: every commit the pull
// request would land, oldest first as gh lists them — the story of the branch,
// which the diff never tells. A row is its subject line and clicking it opens
// the message body, so a branch of fifteen commits stays a list you can scan.
//
// It is also where the account lich runs gh as and the identity the commits
// landed under are finally compared (commit-authors.ts): the two drift in
// silence, and this is the last surface before a merge makes the wrong author
// permanent.
export function PullsCommits({
  commits,
  authorLogin,
}: {
  commits: PullRequestCommit[] | null
  authorLogin: string
}) {
  if (!commits || commits.length === 0) {
    return <Notice className="px-6 py-5 text-sm">No commits.</Notice>
  }
  const notice = commitAuthorNotice(commits, authorLogin)
  return (
    <div className="flex flex-col py-1">
      {notice && (
        <p className="mx-6 mb-1 mt-2 border-l-2 border-amber-500 pl-3 text-xs leading-relaxed text-muted-foreground">
          {notice}
        </p>
      )}
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
    <div>
      {/* The whole row is the affordance — a body is read by clicking the commit
          it belongs to, not a control beside it. A one-line commit has nothing
          to open, so its row stays inert rather than offering an empty toggle. */}
      <button
        type="button"
        onClick={() => setOpen(!open)}
        disabled={body === ""}
        aria-expanded={body === "" ? undefined : open}
        className="flex w-full cursor-default items-center gap-2 px-6 py-2 text-left transition-colors hover:bg-accent/50 disabled:hover:bg-transparent"
      >
        <span className="min-w-0 flex-1">
          <span className="block truncate text-sm font-medium" title={commit.headline}>
            {commit.headline}
          </span>
          <span className="block text-xs text-muted-foreground">{commitMetaLine(commit)}</span>
        </span>
        {body !== "" && (
          <ChevronRight
            className={cn(
              "size-3.5 shrink-0 text-muted-foreground transition-transform",
              open && "rotate-90",
            )}
          />
        )}
        <span className="shrink-0 font-mono text-xs text-muted-foreground" title={commit.oid}>
          {commit.oid.slice(0, 7)}
        </span>
      </button>
      {open && (
        <p className="whitespace-pre-wrap px-6 pb-3 text-xs leading-relaxed text-muted-foreground">
          {body}
        </p>
      )}
    </div>
  )
}
