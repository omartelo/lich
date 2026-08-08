import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"

interface NotificationsOptInProps {
  open: boolean
  /** Escape or the backdrop: closed without an answer, so it is put again. */
  onDismiss: () => void
  /** An answer, stored either way — this dialog is asked once. */
  onDecide: (enabled: boolean) => void
}

// The one-time opt-in for desktop notifications, put the first time a session
// would have notified: the user is being asked about something that just
// happened to them, not about a hypothetical at launch.
export function NotificationsOptIn({ open, onDismiss, onDecide }: NotificationsOptInProps) {
  return (
    <Dialog open={open} onOpenChange={(next) => !next && onDismiss()}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Notify you when a session needs you?</DialogTitle>
          <DialogDescription>
            A session just asked for your input while you were somewhere else. lich can raise a
            desktop notification when that happens, naming the session and its project, so you can
            start something and walk away from the window. Sessions you are looking at never notify.
            You can change this any time in Settings › Notifications.
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="ghost" onClick={() => onDecide(false)}>
            No thanks
          </Button>
          <Button onClick={() => onDecide(true)}>Notify me</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
