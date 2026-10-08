import { useEffect, useState } from "react"
import type { Issue } from "@/lib/api-types"
import { toBranchName } from "@/lib/git/branch-name"
import { issueName, parseIssueRef } from "@/lib/issue"
import { ProjectService } from "@/lib/rpc"
import { errorText } from "@/lib/utils"

// How long the field has to settle before the issue behind a reference is
// looked up. Every keystroke of "#381" is a valid reference on its way to the
// one that was meant, so without this one issue costs three gh calls.
const ISSUE_LOOKUP_MS = 400

export interface WorktreeName {
  name: string
  setName: (name: string) => void
  /** The issue GitHub answered for a typed "#N", once it has. */
  issue: Issue | null
  issueError: string
  issueRef: number | null
  lookingUp: boolean
  /** The branch the typed name creates, "" when nothing usable was typed. */
  newBranch: string
}

// useWorktreeName is the name half of the new-worktree dialogs: what was typed,
// the issue a "#N" names, and the branch all of that turns into.
// WorktreeNameField draws it.
export function useWorktreeName(projectPath: string, open: boolean): WorktreeName {
  const [name, setName] = useState("")
  const [issue, setIssue] = useState<Issue | null>(null)
  const [issueError, setIssueError] = useState("")
  // The issue the field names, if it names one. The reference is what was
  // typed; the issue is what GitHub answered for it, and only that carries the
  // title the branch is named after.
  const issueRef = parseIssueRef(name)

  useEffect(() => {
    if (open) {
      setName("")
    }
  }, [open])

  // The issue behind the typed reference, looked up while the field is still
  // being typed in. A failure is shown but never blocks: the branch under the
  // field is still a branch, and creating the worktree without the issue's text
  // is a worse outcome than not creating it at all only if lich decides so.
  useEffect(() => {
    setIssue(null)
    setIssueError("")
    if (issueRef === null) {
      return
    }
    let stale = false
    const timer = setTimeout(() => {
      ProjectService.Issue(projectPath, issueRef)
        .then((found) => {
          if (!stale) {
            setIssue(found)
          }
        })
        .catch((err: unknown) => {
          if (!stale) {
            setIssueError(errorText(err))
          }
        })
    }, ISSUE_LOOKUP_MS)
    return () => {
      stale = true
      clearTimeout(timer)
    }
  }, [issueRef, projectPath])

  return {
    name,
    setName,
    issue,
    issueError,
    issueRef,
    lookingUp: issueRef !== null && !issue && !issueError,
    // What the typed name creates: itself when git takes it, a slug of the
    // first few words when it is a phrase, blank when nothing usable was typed.
    //
    // A reference is never named by the "#381" that was typed, resolved or not.
    // git accepts "#381" as a branch name, and most shells read the "#" as the
    // start of a comment — `git checkout #381` is `git checkout`. So the issue
    // names it once GitHub has answered, and its bare number until then, which
    // is also what is left when the lookup fails and the worktree is made anyway.
    newBranch: toBranchName(issue ? issueName(issue) : issueRef !== null ? String(issueRef) : name),
  }
}
