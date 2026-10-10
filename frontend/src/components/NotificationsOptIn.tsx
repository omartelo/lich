import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { useT } from "@/lib/i18n/i18n"

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
  const t = useT()
  return (
    <Dialog open={open} onOpenChange={(next) => !next && onDismiss()}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{t("shell.notificationsOptIn.title")}</DialogTitle>
          <DialogDescription>{t("shell.notificationsOptIn.description")}</DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="ghost" onClick={() => onDecide(false)}>
            {t("shell.notificationsOptIn.noThanks")}
          </Button>
          <Button onClick={() => onDecide(true)}>{t("shell.notificationsOptIn.notifyMe")}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
