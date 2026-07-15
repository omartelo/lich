import type { ReactNode } from "react"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"

interface ConfirmDialogProps {
  open: boolean
  /** Called on Cancel and on any outside dismiss (Escape, backdrop). */
  onCancel: () => void
  title: string
  description: ReactNode
  /** Label of the dismiss button. Defaults to "Cancel". */
  cancelLabel?: string
  /** The action buttons, rendered after the shared Cancel. */
  children: ReactNode
}

// ConfirmDialog is the shared shape of every confirmation modal: title,
// description, then a Cancel button followed by the caller's action buttons.
// Callers keep their domain copy (what is being confirmed) and pass only the
// actions that differ.
export function ConfirmDialog({
  open,
  onCancel,
  title,
  description,
  cancelLabel = "Cancel",
  children,
}: ConfirmDialogProps) {
  return (
    <Dialog open={open} onOpenChange={(next) => !next && onCancel()}>
      <DialogContent className="sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription className="break-words">
            {description}
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="ghost" onClick={onCancel}>
            {cancelLabel}
          </Button>
          {children}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
