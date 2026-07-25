import { Notice } from "@/components/common/Notice"
import type { PullRequestCommit } from "@/lib/api-types"

// PullsCommits is the "Commits" tab of the Pulls screen: every commit the pull
// request would land, oldest first as gh lists them, subject and body as git
// itself shows a message — the story of the branch, which the diff never tells.
export function PullsCommits({ commits }: { commits: PullRequestCommit[] | null }) {
  if (!commits || commits.length === 0) {
    return <Notice className="px-6 py-5 text-sm">No commits.</Notice>
  }
  return (
    <div className="flex max-w-3xl flex-col gap-6 px-6 py-5">
      {commits.map((commit) => (
        <CommitEntry key={commit.oid} commit={commit} />
      ))}
    </div>
  )
}

function CommitEntry({ commit }: { commit: PullRequestCommit }) {
  const body = commit.body.trim()
  const meta = [commit.author, commitDate(commit.date)].filter(Boolean).join(" · ")
  return (
    <div className="flex flex-col gap-1.5">
      <div className="flex items-baseline gap-3">
        <h2 className="min-w-0 flex-1 text-sm font-medium leading-snug">{commit.headline}</h2>
        <span className="shrink-0 font-mono text-xs text-muted-foreground" title={commit.oid}>
          {commit.oid.slice(0, 7)}
        </span>
      </div>
      {meta && <p className="text-xs text-muted-foreground">{meta}</p>}
      {body && (
        <p className="mt-0.5 whitespace-pre-wrap text-xs leading-relaxed text-muted-foreground">
          {body}
        </p>
      )}
    </div>
  )
}

// The day is what dates a commit against the others; the hour is noise beside
// the message. An unparseable stamp drops out of the meta line entirely.
function commitDate(iso: string): string {
  const at = new Date(iso)
  return Number.isNaN(at.getTime()) ? "" : at.toLocaleDateString()
}
