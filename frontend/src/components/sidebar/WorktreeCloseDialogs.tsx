import { useEffect, useState } from "react"
import { Button } from "@/components/ui/button"
import { ConfirmDialog } from "@/components/ConfirmDialog"
import { useGitStatus } from "@/lib/git/use-git-status"
import { ProjectService } from "@/lib/rpc"
import type { RaceRival } from "@/lib/session/agent-race"
import type { Session } from "@/lib/session/sessions"
import { count } from "@/lib/utils"
import type { KeepWinner, WorktreeClose } from "./useWorktreeClose"

interface RunningSessionDialogProps {
  /** The session with a turn still in flight, or null when the dialog is hidden. */
  session: Session | null
  onCancel: () => void
  /** Close it anyway, killing whatever the agent was in the middle of. */
  onCloseAnyway: () => void
}

// RunningSessionDialog is the question a close raises while the agent is still
// working, or blocked on a prompt nobody answered: the card's × kills the PTY,
// and neither the turn nor the answer it was waiting for survives that.
function RunningSessionDialog({ session, onCancel, onCloseAnyway }: RunningSessionDialogProps) {
  return (
    <ConfirmDialog
      open={session !== null}
      onCancel={onCancel}
      title="Session is still running"
      description={
        <>
          <span className="font-medium">{session?.label}</span> is mid-turn. Closing it stops the
          process where it stands; the turn in flight is lost.
        </>
      }
    >
      <Button variant="destructive" onClick={onCloseAnyway}>
        Close anyway
      </Button>
    </ConfirmDialog>
  )
}

interface CloseWorktreeDialogProps {
  /** The worktree session being closed, or null when the dialog is hidden. */
  session: Session | null
  /** Whether lich adopted that checkout rather than creating it. */
  adopted: boolean
  onCancel: () => void
  /** Close the session, leaving the worktree on disk. */
  onKeep: () => void
  /** Close the session and remove the worktree checkout (branch stays). */
  onRemove: () => void
}

// CloseWorktreeDialog asks what to do with the worktree a closing session lives
// in: keep it on disk (it reappears in the new-worktree picker) or remove the
// checkout via git. The branch is never deleted either way.
//
// A checkout lich adopted is removable too, but the directory is the user's own
// — made outside lich and only listed by it — so the wording names the absolute
// path and says lich did not make it. That sentence is the acknowledgement the
// backend asks for (project.RemoveWorktree): without it a caller that never
// showed the path is refused.
function CloseWorktreeDialog({
  session,
  adopted,
  onCancel,
  onKeep,
  onRemove,
}: CloseWorktreeDialogProps) {
  const path = <span className="break-all font-mono">{session?.path}</span>
  return (
    <ConfirmDialog
      open={session !== null}
      onCancel={onCancel}
      title="Close worktree session"
      description={
        adopted ? (
          <>
            lich did not create the worktree at {path}. Keep it, or remove the checkout? Removing
            deletes that directory but keeps its branch.
          </>
        ) : (
          <>
            Keep or remove the worktree at {path}? Removing deletes the checkout but keeps its
            branch.
          </>
        )
      }
    >
      <Button variant="outline" onClick={onKeep}>
        Keep worktree
      </Button>
      <Button variant="destructive" onClick={onRemove}>
        Remove worktree
      </Button>
    </ConfirmDialog>
  )
}

interface ForceRemoveWorktreeDialogProps {
  /** The dirty worktree session pending forced removal, or null when hidden. */
  session: Session | null
  onCancel: () => void
  /** Remove the worktree with --force, discarding its uncommitted changes. */
  onForceRemove: () => void
}

// ForceRemoveWorktreeDialog is the second confirmation shown when the worktree
// picked for removal has uncommitted changes: git refuses a plain remove, so
// proceeding means --force and the changes are gone for good.
function ForceRemoveWorktreeDialog({
  session,
  onCancel,
  onForceRemove,
}: ForceRemoveWorktreeDialogProps) {
  return (
    <ConfirmDialog
      open={session !== null}
      onCancel={onCancel}
      title="Worktree has uncommitted changes"
      description={
        <>
          The worktree at <span className="break-all font-mono">{session?.path}</span> contains
          uncommitted changes. Removing it will discard them permanently. The branch is kept.
        </>
      }
    >
      <Button variant="destructive" onClick={onForceRemove}>
        Discard changes and remove
      </Button>
    </ConfirmDialog>
  )
}

interface RivalLineProps {
  rival: RaceRival
  running: boolean
}

// One checkout the keep removes, with what is lost with it: a turn in flight,
// the work it never committed, and whose directory it is when lich did not
// make it. The adopted probe reads as adopted until answered, for the reason
// useWorktreeClose gives: the sentence that says more goes up first.
function RivalLine({ rival, running }: RivalLineProps) {
  const status = useGitStatus(rival.path)
  const [adopted, setAdopted] = useState(true)
  useEffect(() => {
    let stale = false
    void ProjectService.WorktreeAdopted(rival.path)
      .catch(() => true)
      .then((answer) => {
        if (!stale) {
          setAdopted(answer)
        }
      })
    return () => {
      stale = true
    }
  }, [rival.path])
  const lost = [
    running ? "turn running" : "",
    status && status.files > 0 ? `${count(status.files, "file")} uncommitted` : "",
    adopted ? "not created by lich" : "",
  ].filter(Boolean)
  return (
    <span className="flex flex-col rounded-md bg-accent/50 px-2 py-1.5">
      <span className="break-all font-mono text-xs text-foreground">{rival.path}</span>
      {lost.length > 0 && <span className="text-xs text-tone-wait">{lost.join(" · ")}</span>}
    </span>
  )
}

interface KeepWinnerDialogProps {
  keep: KeepWinner | null
  onCancel: () => void
  onConfirm: () => void
}

// KeepWinnerDialog is the one confirmation behind "keep this, remove the
// others": every rival checkout is named with what goes with it, and the
// button removes them all, forced. A dirty checkout gets no second question
// here, unlike a single close, because this list is that question for all of
// them at once.
function KeepWinnerDialog({ keep, onCancel, onConfirm }: KeepWinnerDialogProps) {
  const total = keep?.rivals.length ?? 0
  return (
    <ConfirmDialog
      open={keep !== null}
      onCancel={onCancel}
      title={
        keep?.winner && !keep.consolidated
          ? `Keep ${keep.winner.label}?`
          : `Remove the ${keep?.folder} race?`
      }
      description={
        <>
          Removes {keep?.winner ? "the other" : "the"} {count(total, "worktree")} in{" "}
          <span className="font-medium">{keep?.folder}</span>, with their sessions. Branches stay;
          uncommitted work in them is discarded and exists nowhere else.
          {keep?.consolidated && <> {keep.winner?.label} is kept.</>}
          <span className="mt-3 flex flex-col gap-1">
            {keep?.rivals.map((rival) => (
              <RivalLine
                key={rival.path}
                rival={rival}
                running={rival.sessions.some((session) => keep.running.includes(session.id))}
              />
            ))}
          </span>
        </>
      }
    >
      <Button variant="destructive" onClick={onConfirm}>
        Remove {count(total, "worktree")}
      </Button>
    </ConfirmDialog>
  )
}

// WorktreeCloseDialogs mounts every question one close flow can raise. It is a
// component rather than three tags at each call site because the flow now has
// two entry points — the sidebar card's × and the exit banner inside the
// terminal — and each owns its own instance of the state machine.
export function WorktreeCloseDialogs({ close }: { close: WorktreeClose }) {
  return (
    <>
      <RunningSessionDialog
        session={close.pendingRunning}
        onCancel={close.cancel}
        onCloseAnyway={close.closeAnyway}
      />
      <CloseWorktreeDialog
        session={close.pendingClose}
        adopted={close.pendingAdopted}
        onCancel={close.cancel}
        onKeep={close.keep}
        onRemove={close.remove}
      />
      <ForceRemoveWorktreeDialog
        session={close.pendingForce}
        onCancel={close.cancel}
        onForceRemove={close.forceRemove}
      />
      {/* Mounted only while asked, unlike the three above: it is the one with
          a probe and a git status per row, and a closed one would still repaint
          on every project switch (render-budget.test.tsx). */}
      {close.pendingKeep && (
        <KeepWinnerDialog
          keep={close.pendingKeep}
          onCancel={close.cancel}
          onConfirm={close.keepWinner}
        />
      )}
    </>
  )
}
