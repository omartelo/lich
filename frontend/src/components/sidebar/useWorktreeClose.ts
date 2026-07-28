import { useState } from "react"
import { toast } from "sonner"
import { ProjectService, Store } from "@/lib/rpc"
import { isLastWorktreeSession, type Session } from "@/lib/session/sessions"
import { useProjects } from "@/providers/projects"
import { errorText } from "@/lib/utils"

export interface WorktreeClose {
  /** Close a session. The last one in a worktree opens the keep-or-remove
   * question instead, since it is the last thing holding that checkout. */
  requestClose: (session: Session) => void
  /** The session whose keep-or-remove dialog is open, or null. */
  pendingClose: Session | null
  /** The dirty worktree waiting on a --force confirmation, or null. */
  pendingForce: Session | null
  /** Dismiss whichever dialog is open, changing nothing. */
  cancel: () => void
  /** Close the session but park its row, so the worktree can be resumed. */
  keep: () => void
  /** Close the session and remove the checkout; a dirty one asks again first. */
  remove: () => void
  /** Confirmed: remove the dirty checkout, discarding what was never committed. */
  forceRemove: () => void
}

// useWorktreeClose owns the two-step question closing a worktree's last session
// raises — keep the checkout for later, or remove it, and if it is dirty, say so
// once more before --force discards work that lives nowhere else. Every step
// past the first is a dialog, which is why this is a state machine and not a
// handler: the answer arrives renders later than the click.
//
// Closing a session that is not the last of its worktree never gets here; it
// goes straight through, because nothing is at stake.
export function useWorktreeClose(
  projectId: string,
  projectPath: string,
  sessions: Session[],
): WorktreeClose {
  const { closeSession, keepSession } = useProjects()
  const [pendingClose, setPendingClose] = useState<Session | null>(null)
  const [pendingForce, setPendingForce] = useState<Session | null>(null)

  // Close first so the PTY running inside the worktree dies before git tries
  // to remove it. A refused removal surfaces as a toast; the checkout stays on
  // disk and reappears in the new-worktree picker.
  const closeAndRemove = (session: Session, force: boolean) => {
    closeSession(projectId, session.id)
    // The checkout is going away, so no parked row for it may linger — one would
    // otherwise resurface a resume against a worktree that no longer exists.
    void Store.PurgeWorktreeSessions(projectId, session.path ?? "")
    ProjectService.RemoveWorktree(projectPath, session.path ?? "", force).catch((err: unknown) => {
      toast.error(`Failed to remove worktree: ${errorText(err)}`)
    })
  }

  return {
    pendingClose,
    pendingForce,

    requestClose(session) {
      if (isLastWorktreeSession(sessions, session)) {
        setPendingClose(session)
        return
      }
      closeSession(projectId, session.id)
    },

    cancel() {
      setPendingClose(null)
      setPendingForce(null)
    },

    keep() {
      if (pendingClose) {
        keepSession(projectId, pendingClose.id)
      }
      setPendingClose(null)
    },

    remove() {
      const session = pendingClose
      setPendingClose(null)
      if (!session?.path) {
        return
      }
      // A dirty worktree needs a second confirmation before --force discards its
      // changes. A failed check falls through to the plain remove, whose own
      // refusal surfaces as a toast.
      void ProjectService.WorktreeDirty(session.path)
        .catch(() => false)
        .then((dirty) => {
          if (dirty) {
            setPendingForce(session)
            return
          }
          closeAndRemove(session, false)
        })
    },

    forceRemove() {
      const session = pendingForce
      setPendingForce(null)
      if (session?.path) {
        closeAndRemove(session, true)
      }
    },
  }
}
