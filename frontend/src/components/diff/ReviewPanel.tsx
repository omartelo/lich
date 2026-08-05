import { useCallback, useEffect, useRef, useState } from "react"
import { toast } from "sonner"
import { Notice } from "@/components/common/Notice"
import { discardTargets, parseDiff, type DiffFile } from "@/lib/git/diff"
import { addReviewComment } from "@/lib/review-comments"
import { ProjectService } from "@/lib/rpc"
import { useActiveSession } from "@/lib/session/use-active-session"
import { useGitStatus } from "@/lib/git/use-git-status"
import { useInject } from "@/lib/use-inject"
import { errorText } from "@/lib/utils"
import { CommentBatch } from "./CommentBatch"
import type { DiffBulk } from "./diff-bulk"
import { DiscardDialog } from "./DiscardDialog"
import { FileDiff } from "./FileDiff"

// How often the diff *text* is re-read while the panel is open. The status
// counts invalidate it the moment they move, but they cannot see an edit that
// leaves them alone: replacing text on a line that was already modified keeps
// files/added/deleted exactly where they were, and HEAD does not move either.
// With no read of its own the panel held that stale diff for as long as the
// counts stood still — not one tick, but indefinitely.
const DIFF_POLL_MS = 2_000

// ReviewPanel is the Review tab's body: the active session's uncommitted diff,
// one collapsible file at a time. Context-menu actions write file/line
// references into the session's PTY, mirroring the footer's attach-file button,
// or hold a comment on the selection for the batch at the panel's foot.
// It follows the active session — a worktree session reviews its checkout, not
// the project root. The dock (RightDock) owns the surrounding chrome: width,
// full screen, the tab bar and the close button.
export function ReviewPanel({ bulk }: { bulk: DiffBulk }) {
  const { sessionId, path } = useActiveSession()
  const inject = useInject(sessionId)
  const status = useGitStatus(path)
  const [files, setFiles] = useState<DiffFile[] | null>(null)
  const [failed, setFailed] = useState(false)
  const [pendingDiscard, setPendingDiscard] = useState<DiffFile | null>(null)

  // The text behind what is on screen, and the id of the newest read — a reply
  // carrying an older one is dropped, so switching checkouts mid-flight cannot
  // land the previous one's diff.
  const lastText = useRef<string | null>(null)
  const seq = useRef(0)

  // Publishing only on a changed text is what lets this be polled at all: an
  // identical read leaves `files` — and with it every mounted CodeMirror view,
  // its selection and its scroll position — untouched.
  const refresh = useCallback(async () => {
    if (!path) {
      return
    }
    const mine = ++seq.current
    try {
      const text = await ProjectService.DiffText(path)
      if (mine !== seq.current) {
        return
      }
      setFailed(false)
      if (text === lastText.current) {
        return
      }
      lastText.current = text
      setFiles(parseDiff(text))
    } catch {
      if (mine !== seq.current) {
        return
      }
      lastText.current = null
      setFiles([])
      setFailed(true)
    }
  }, [path])

  // Two signals feeding one read: the status counts move the instant a file is
  // touched, so they invalidate immediately, and the interval covers the edits
  // they are blind to (see DIFF_POLL_MS).
  useEffect(() => {
    void refresh()
    const timer = setInterval(() => void refresh(), DIFF_POLL_MS)
    return () => clearInterval(timer)
  }, [refresh, status?.files, status?.added, status?.deleted])

  // Reverting a rename touches both paths (new removed, old restored); the
  // panel refreshes immediately instead of waiting for the next poll tick.
  const discard = async () => {
    const file = pendingDiscard
    setPendingDiscard(null)
    if (!file) {
      return
    }
    try {
      for (const rel of discardTargets(file)) {
        await ProjectService.DiscardFile(path, rel)
      }
    } catch (err: unknown) {
      toast.error(`Failed to discard changes: ${errorText(err)}`)
    }
    void refresh()
  }

  return (
    <div className="flex h-full flex-col">
      <div className="flex-1 overflow-y-auto">
        <PanelBody
          files={files}
          failed={failed}
          onInject={inject}
          onSessionComment={(file, lines, text) => addReviewComment(path, file, lines, text)}
          onDiscard={setPendingDiscard}
          bulk={bulk}
        />
      </div>
      <CommentBatch target={path} onInject={inject} />
      <DiscardDialog
        file={pendingDiscard}
        onCancel={() => setPendingDiscard(null)}
        onDiscard={() => void discard()}
      />
    </div>
  )
}

interface PanelBodyProps {
  files: DiffFile[] | null
  failed: boolean
  onInject: (text: string) => void
  onSessionComment: (path: string, lines: string, text: string) => void
  onDiscard: (file: DiffFile) => void
  bulk: DiffBulk
}

function PanelBody({ files, failed, onInject, onSessionComment, onDiscard, bulk }: PanelBodyProps) {
  if (failed) {
    return <Notice>Not a git repository</Notice>
  }
  if (files === null) {
    return <Notice>Loading…</Notice>
  }
  if (files.length === 0) {
    return <Notice>No uncommitted changes</Notice>
  }
  return (
    <div className="flex flex-col p-2 [&>section:not(:first-child)]:mt-2.5 [&>section:not(:first-child)]:border-t [&>section:not(:first-child)]:border-border [&>section:not(:first-child)]:pt-2.5">
      {files.map((file) => (
        <FileDiff
          key={file.newPath}
          file={file}
          onInject={onInject}
          onSessionComment={onSessionComment}
          onDiscard={() => onDiscard(file)}
          bulk={bulk}
        />
      ))}
    </div>
  )
}
