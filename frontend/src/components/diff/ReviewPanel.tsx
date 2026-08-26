import { useCallback, useEffect, useRef, useState } from "react"
import { toast } from "sonner"
import { Notice } from "@/components/common/Notice"
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group"
import type { LastTurn } from "@/lib/api-types"
import { onAppEvent } from "@/lib/app-events"
import { readDiffSource, writeDiffSource, type DiffSource } from "@/lib/dock-prefs"
import { discardTargets, parseDiff, type DiffFile } from "@/lib/git/diff"
import { lastTurnNotice } from "@/lib/git/last-turn"
import { addReviewComment } from "@/lib/review-comments"
import { ProjectService, Terminal } from "@/lib/rpc"
import { useActiveSession } from "@/lib/session/use-active-session"
import { formatAge, subscribeAge } from "@/lib/session/session-age"
import { isIdEvent, TURN_EVENT } from "@/lib/session/session-events"
import { useSessionEverReported } from "@/lib/session/use-session-status"
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

// Said in full on the option itself, because the card cannot: the pair of
// snapshots brackets wall-clock time, so every hand that touched the checkout
// while the turn ran is inside it. Nothing here can attribute a line, and the
// wording must not imply otherwise.
const LAST_TURN_HINT =
  "What changed on disk while the last turn ran — including edits from a formatter, your editor, or you."

// ReviewPanel is the Review tab's body: the active session's diff, one
// collapsible file at a time, read either from the working tree or from the
// session's last finished turn. Context-menu actions write file/line references
// into the session's PTY, mirroring the footer's attach-file button, or hold a
// comment on the selection for the batch at the panel's foot.
// It follows the active session — a worktree session reviews its checkout, not
// the project root. The dock (RightDock) owns the surrounding chrome: width,
// full screen, the tab bar and the close button.
export function ReviewPanel({ bulk }: { bulk: DiffBulk }) {
  const { sessionId, path } = useActiveSession()
  const inject = useInject(sessionId)
  const status = useGitStatus(path)
  // The source switch is earned, not assumed: a provider that never reports its
  // state has no turn to bracket, so it is offered the working tree alone.
  const switchable = useSessionEverReported(sessionId)
  // The source the reviewer picked, read back on every mount because the dock
  // has no shortage of them: it is a ternary between two component types, so
  // each flip to the Code tab unmounts this panel whole (dock-prefs).
  const [wanted, setWanted] = useState<DiffSource>(readDiffSource)
  // A source the session cannot answer for must never be the one on screen: it
  // would sit on an empty panel with no control anywhere to leave it by. So the
  // guard makes what is *shown*, and nothing writes "worktree" back over the
  // choice — `switchable` starts false after a reload and turns true only when
  // the session next reports, so a reset would fire first and throw the
  // remembered choice away before the switch ever appeared.
  const source: DiffSource = switchable ? wanted : "worktree"
  const changeSource = (next: DiffSource) => {
    writeDiffSource(next)
    setWanted(next)
  }
  const [files, setFiles] = useState<DiffFile[] | null>(null)
  const [failed, setFailed] = useState(false)
  const [turnState, setTurnState] = useState<LastTurn["state"] | null>(null)
  const [endedAt, setEndedAt] = useState<number | null>(null)
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
      // The working tree wears the same shape rather than branching the whole
      // read: it is always "ok", it has no window to date, and every line below
      // then reads one source.
      const turn: LastTurn =
        source === "turn"
          ? await Terminal.LastTurnDiff(sessionId)
          : { state: "ok", diff: await ProjectService.DiffText(path) }
      if (mine !== seq.current) {
        return
      }
      setFailed(false)
      setTurnState(turn.state)
      setEndedAt(turn.endedAt ?? null)
      const text = turn.diff ?? ""
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
  }, [path, sessionId, source])

  // Switching source — or checkout — invalidates the held text rather than
  // trusting the identity check below it: two sources legitimately produce the
  // same string, most often "", and the short-circuit would leave the previous
  // source's files on screen under the new source's name.
  useEffect(() => {
    lastText.current = null
    setFiles(null)
    setTurnState(null)
    setEndedAt(null)
  }, [source, path, sessionId])

  // Two signals feeding one read: the status counts move the instant a file is
  // touched, so they invalidate immediately, and the interval covers the edits
  // they are blind to (see DIFF_POLL_MS).
  //
  // A finished turn has neither problem — it cannot change once it is over — so
  // that source is not polled at all. TURN_EVENT is what moves it, and it has to
  // be that rather than the session's state: the record is filed on a snapshot
  // worker well after the `done` that closed the turn, so a read taken on the
  // state report answers with the turn before this one.
  useEffect(() => {
    void refresh()
    if (source === "turn") {
      return onAppEvent(TURN_EVENT, (data) => {
        if (isIdEvent(data) && data.id === sessionId) {
          void refresh()
        }
      })
    }
    const timer = setInterval(() => void refresh(), DIFF_POLL_MS)
    return () => clearInterval(timer)
  }, [refresh, source, sessionId, status?.files, status?.added, status?.deleted])

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
      {switchable && <SourceRow source={source} onSource={changeSource} endedAt={endedAt} />}
      <div className="flex-1 overflow-y-auto">
        <PanelBody
          source={source}
          turnState={turnState}
          files={files}
          failed={failed}
          onInject={inject}
          onSessionComment={(file, lines, text) => addReviewComment(path, file, lines, text)}
          // Discarding is an edit to the working tree, so it belongs to the
          // working tree's own view. Offered on a finished turn it would read as
          // an undo for that turn, which is not what it does.
          onDiscard={source === "worktree" ? setPendingDiscard : undefined}
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

interface SourceRowProps {
  source: DiffSource
  onSource: (source: DiffSource) => void
  /** When the shown turn's window closed (unix ms), or null when none is. */
  endedAt: number | null
}

// SourceRow is the strip above the file list holding the source switch — the
// same shape the Code tab gives its filter field, so the two tabs read as
// siblings rather than as two different panels.
function SourceRow({ source, onSource, endedAt }: SourceRowProps) {
  const age = useAge(source === "turn" ? endedAt : null)
  return (
    <div className="flex shrink-0 items-center gap-2 border-b border-border p-1.5">
      <ToggleGroup
        value={[source]}
        onValueChange={(next) => next[0] && onSource(next[0] as DiffSource)}
        spacing={1}
        aria-label="Which changes to show"
        className="border border-border p-[3px]"
      >
        <ToggleGroupItem value="worktree" size="sm" className="h-6 px-2.5 text-xs">
          Working tree
        </ToggleGroupItem>
        <ToggleGroupItem
          value="turn"
          size="sm"
          className="h-6 px-2.5 text-xs"
          title={LAST_TURN_HINT}
        >
          Last turn
        </ToggleGroupItem>
      </ToggleGroup>
      {/* The one thing the panel can state without claiming authorship: when
          the window closed. Absent while there is no window to date, which is
          also what tells an empty turn from an unrecorded one at a glance. */}
      {age !== "" && (
        <span className="ml-auto shrink-0 pr-1 text-[0.6875rem] text-muted-foreground">
          ended {age} ago
        </span>
      )}
    </div>
  )
}

// useAge renders how long ago a moment was, on the sidebar's shared clock — one
// timer for every readout on screen, and none at all while there is nothing to
// count (see subscribeAge).
function useAge(from: number | null): string {
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    if (from === null) {
      return
    }
    return subscribeAge(from, setNow)
  }, [from])
  return from === null ? "" : formatAge(now - from)
}

interface PanelBodyProps {
  source: DiffSource
  /** The last turn's own answer, null while showing the working tree or before
   * the first read lands. */
  turnState: LastTurn["state"] | null
  files: DiffFile[] | null
  failed: boolean
  onInject: (text: string) => void
  onSessionComment: (path: string, lines: string, text: string) => void
  /** Absent when discarding does not belong to this source. */
  onDiscard?: (file: DiffFile) => void
  bulk: DiffBulk
}

function PanelBody({
  source,
  turnState,
  files,
  failed,
  onInject,
  onSessionComment,
  onDiscard,
  bulk,
}: PanelBodyProps) {
  if (failed) {
    // The two sources fail for different reasons, and the working tree's answer
    // — read for a path that is not a checkout — says nothing true about a turn
    // lich could not render.
    return (
      <Notice>{source === "turn" ? "Could not read the last turn" : "Not a git repository"}</Notice>
    )
  }
  if (files === null) {
    return <Notice>Loading…</Notice>
  }
  if (source === "turn") {
    // A turn that changed nothing and a turn nobody recorded are different
    // answers, and each says which one it is. Conflating them would have the
    // panel report "nothing happened" for a snapshot it simply lost, which is
    // why the weighing is pure and tested (lastTurnNotice).
    const notice = lastTurnNotice(turnState, files.length)
    if (notice === "empty") {
      return (
        <Notice>
          <span className="block text-foreground">Nothing changed in this window.</span>
          No file on disk moved between the last turn starting and ending.
        </Notice>
      )
    }
    if (notice === "unrecorded") {
      return (
        <Notice>
          <span className="block text-foreground">No last turn recorded.</span>
          This fills in when a turn ends here.
        </Notice>
      )
    }
  } else if (files.length === 0) {
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
          onDiscard={onDiscard && (() => onDiscard(file))}
          bulk={bulk}
        />
      ))}
    </div>
  )
}
