import { useRef, useState } from "react"
import { toast } from "sonner"
import { ProjectService, Store } from "@/lib/rpc"
import { closeIntent } from "@/lib/session/close-intent"
import { runningSessions } from "@/lib/session/use-session-status"
import type { Session } from "@/lib/session/sessions"
import { useProjects } from "@/providers/projects"
import { errorText } from "@/lib/utils"

export interface WorktreeClose {
  /** Close a session, asking whatever its state calls for first (closeIntent). */
  requestClose: (session: Session) => void
  /** The session whose mid-turn confirmation is open, or null. */
  pendingRunning: Session | null
  /** Confirmed: close the session even though a turn is running in it. */
  closeAnyway: () => void
  /** The session whose keep-or-remove dialog is open, or null. */
  pendingClose: Session | null
  /**
   * Whether that checkout is one lich adopted rather than created, in which case
   * it is the user's directory and removal is not on offer. True until the
   * backend says otherwise, so the destructive answer never appears on a guess.
   */
  pendingAdopted: boolean
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

// useWorktreeClose owns every question a close raises — is a turn still running
// in there, keep the checkout for later or remove it, and if it is dirty, say so
// once more before --force discards work that lives nowhere else. Every step
// past the first is a dialog, which is why this is a state machine and not a
// handler: the answer arrives renders later than the click.
//
// A close with nothing at stake never opens one; it goes straight through.
export function useWorktreeClose(
  projectId: string,
  projectPath: string,
  sessions: Session[],
): WorktreeClose {
  const { closeSession, discardSession, keepSession } = useProjects()
  const [pendingRunning, setPendingRunning] = useState<Session | null>(null)
  const [pendingClose, setPendingClose] = useState<Session | null>(null)
  const [pendingAdopted, setPendingAdopted] = useState(true)
  const [pendingForce, setPendingForce] = useState<Session | null>(null)
  const adoptedProbe = useRef(0)

  // Close first so the PTY running inside the worktree dies before git tries
  // to remove it. A refused removal surfaces as a toast; the checkout stays on
  // disk and reappears in the new-worktree picker.
  //
  // discardSession, not closeSession: the checkout is going away in the same
  // breath, so an undo would restore a card rooted in a directory that no
  // longer exists.
  const closeAndRemove = (session: Session, force: boolean) => {
    discardSession(projectId, session.id)
    // The checkout is going away, so no parked row for it may linger — one would
    // otherwise resurface a resume against a worktree that no longer exists.
    void Store.PurgeWorktreeSessions(projectId, session.path ?? "")
    ProjectService.RemoveWorktree(projectPath, session.path ?? "", force).catch((err: unknown) => {
      toast.error(`Failed to remove worktree: ${errorText(err)}`)
    })
  }

  // Each step of the close, taken as closeIntent decides it. The card hides
  // every close affordance while pinned; the refusal here is the contract behind
  // them, so no dialog can open on a pinned session either.
  const step = (session: Session, running: boolean) => {
    switch (closeIntent(session, sessions, running)) {
      case "refuse":
        return
      case "confirm-running":
        setPendingRunning(session)
        return
      case "ask-worktree": {
        // Asked as the dialog opens rather than at the click on Remove: an
        // adopted checkout has no removal to offer, and a button that only ever
        // ends in a refusal is worse than the one that is not there. A failed
        // check reads as adopted — the safe half of the answer.
        setPendingAdopted(true)
        setPendingClose(session)
        const sequence = ++adoptedProbe.current
        void ProjectService.WorktreeAdopted(session.path ?? "")
          .catch(() => true)
          .then((adopted) => {
            if (sequence === adoptedProbe.current) {
              setPendingAdopted(adopted)
            }
          })
        return
      }
      case "close":
        closeSession(projectId, session.id)
    }
  }

  return {
    pendingRunning,
    pendingClose,
    pendingAdopted,
    pendingForce,

    requestClose(session) {
      // Read at the click rather than subscribed to: what matters is whether a
      // turn is running at the moment the user asked to close it.
      step(session, runningSessions([session.id]).length > 0)
    },

    closeAnyway() {
      const session = pendingRunning
      setPendingRunning(null)
      if (session) {
        step(session, false)
      }
    },

    cancel() {
      adoptedProbe.current++
      setPendingRunning(null)
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
