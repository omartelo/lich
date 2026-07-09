import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import type { Session } from "@/lib/sessions"

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
    <Dialog open={session !== null} onOpenChange={(open) => !open && onCancel()}>
      <DialogContent className="sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>Close worktree session</DialogTitle>
          <DialogDescription className="break-words">
            Keep or remove the worktree at{" "}
            <span className="font-mono">{session?.path}</span>? Removing deletes
            the checkout but keeps its branch.
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="ghost" onClick={onCancel}>
            Cancel
          </Button>
          <Button variant="outline" onClick={onKeep}>
            Keep worktree
          </Button>
          <Button variant="destructive" onClick={onRemove}>
            Remove worktree
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
