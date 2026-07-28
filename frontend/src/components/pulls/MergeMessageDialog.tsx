import { GitMerge } from "lucide-react"
import type { MergeMethod } from "@/lib/api-types"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"

// MergeEdit carries a pending "edit commit message" merge: which method to run
// and the message the dialog is editing. The screen holds it as null while the
// dialog is closed.
export interface MergeEdit {
  method: MergeMethod
  title: string
  subject: string
  body: string
}

// mergeEditFor seeds the dialog with what GitHub would have written by itself,
// which is what the user is here to adjust rather than to compose: a squash
// takes the pull request's title and body, a merge commit takes git's own
// "Merge pull request #N from branch" and no body.
export function mergeEditFor(
  method: Extract<MergeMethod, "squash" | "merge">,
  detail: { number: number; title: string; body: string; headRefName: string },
): MergeEdit {
  if (method === "squash") {
    return {
      method,
      title: "Squash and merge",
      subject: `${detail.title} (#${detail.number})`,
      body: detail.body,
    }
  }
  return {
    method,
    title: "Create a merge commit",
    subject: `Merge pull request #${detail.number} from ${detail.headRefName}`,
    body: "",
  }
}

interface MergeMessageDialogProps {
  edit: MergeEdit
  merging: boolean
  onChange: (next: MergeEdit) => void
  onCancel: () => void
  onConfirm: () => void
}

export function MergeMessageDialog({
  edit,
  merging,
  onChange,
  onCancel,
  onConfirm,
}: MergeMessageDialogProps) {
  return (
    <Dialog open onOpenChange={(next) => !next && onCancel()}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{edit.title}</DialogTitle>
          <DialogDescription>Edit the commit message, then merge.</DialogDescription>
        </DialogHeader>
        <div className="flex flex-col gap-3">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="merge-subject">Commit message</Label>
            <Input
              id="merge-subject"
              value={edit.subject}
              onChange={(e) => onChange({ ...edit, subject: e.target.value })}
              autoFocus
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="merge-body">Extended description</Label>
            <textarea
              id="merge-body"
              value={edit.body}
              onChange={(e) => onChange({ ...edit, body: e.target.value })}
              rows={6}
              placeholder="Optional"
              className="min-h-24 w-full resize-y rounded-md border border-input bg-transparent px-2.5 py-1.5 text-sm shadow-xs outline-none transition-[color,box-shadow] placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 dark:bg-input/30"
            />
          </div>
        </div>
        <DialogFooter>
          <Button variant="ghost" onClick={onCancel}>
            Cancel
          </Button>
          <Button onClick={onConfirm} disabled={merging || edit.subject.trim() === ""}>
            <GitMerge />
            {merging ? "Merging…" : edit.method === "squash" ? "Squash and merge" : "Merge"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
