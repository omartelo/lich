import { Button } from "@/components/ui/button"
import { ConfirmDialog } from "@/components/ConfirmDialog"
import type { Session } from "@/lib/session/sessions"

interface CloseWorktreeDialogProps {
  /** The worktree session being closed, or null when the dialog is hidden. */
  session: Session | null
  onCancel: () => void
  /** Close the session, leaving the worktree on disk. */
  onKeep: () => void
  /** Close the session and remove the worktree checkout (branch stays). */
  onRemove: () => void
}

// CloseWorktreeDialog asks what to do with the worktree a closing session lives
// in: keep it on disk (it reappears in the new-worktree picker) or remove the
// checkout via git. The branch is never deleted either way.
export function CloseWorktreeDialog({
  session,
  onCancel,
  onKeep,
  onRemove,
}: CloseWorktreeDialogProps) {
  return (
    <ConfirmDialog
      open={session !== null}
      onCancel={onCancel}
      title="Close worktree session"
      description={
        <>
          Keep or remove the worktree at{" "}
          <span className="break-all font-mono">{session?.path}</span>? Removing deletes the
          checkout but keeps its branch.
        </>
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
export function ForceRemoveWorktreeDialog({
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
