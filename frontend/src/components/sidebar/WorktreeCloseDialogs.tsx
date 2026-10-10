import { Trans } from "@/components/common/Trans"
import { Button } from "@/components/ui/button"
import { ConfirmDialog } from "@/components/ConfirmDialog"
import { useT } from "@/lib/i18n/i18n"
import type { Session } from "@/lib/session/sessions"
import type { WorktreeClose } from "./useWorktreeClose"

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
  const t = useT()
  return (
    <ConfirmDialog
      open={session !== null}
      onCancel={onCancel}
      title={t("sidebar.worktreeCloseDialogs.runningTitle")}
      description={
        <Trans
          k="sidebar.worktreeCloseDialogs.runningDescription"
          params={{ label: <span className="font-medium">{session?.label}</span> }}
        />
      }
    >
      <Button variant="destructive" onClick={onCloseAnyway}>
        {t("sidebar.worktreeCloseDialogs.closeAnyway")}
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
  const t = useT()
  const path = <span className="break-all font-mono select-text">{session?.path}</span>
  return (
    <ConfirmDialog
      open={session !== null}
      onCancel={onCancel}
      title={t("sidebar.worktreeCloseDialogs.closeTitle")}
      description={
        adopted ? (
          <Trans k="sidebar.worktreeCloseDialogs.adoptedDescription" params={{ path }} />
        ) : (
          <Trans k="sidebar.worktreeCloseDialogs.createdDescription" params={{ path }} />
        )
      }
    >
      <Button variant="outline" onClick={onKeep}>
        {t("sidebar.worktreeCloseDialogs.keep")}
      </Button>
      <Button variant="destructive" onClick={onRemove}>
        {t("sidebar.worktreeCloseDialogs.remove")}
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
  const t = useT()
  return (
    <ConfirmDialog
      open={session !== null}
      onCancel={onCancel}
      title={t("sidebar.worktreeCloseDialogs.dirtyTitle")}
      description={
        <Trans
          k="sidebar.worktreeCloseDialogs.dirtyDescription"
          params={{
            path: <span className="break-all font-mono select-text">{session?.path}</span>,
          }}
        />
      }
    >
      <Button variant="destructive" onClick={onForceRemove}>
        {t("sidebar.worktreeCloseDialogs.discardAndRemove")}
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
    </>
  )
}
