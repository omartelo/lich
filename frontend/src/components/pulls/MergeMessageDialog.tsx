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
import { Label } from "@/components/ui/label"
import { cn } from "@/lib/utils"
import { commentFieldClass } from "./CommentBox"

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
  const canMerge = !merging && edit.subject.trim() !== ""
  return (
    <Dialog open onOpenChange={(next) => !next && onCancel()}>
      {/* Wider than the app's other dialogs, and bounded by the window rather
          than by the message: what is being edited here is a commit message
          written elsewhere — a conventional-commit subject and a body of
          several bullets — and a dialog sized for a sentence hides most of it
          behind a scrollbar. */}
      <DialogContent className="max-h-[85vh] overflow-y-auto sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>{edit.title}</DialogTitle>
          <DialogDescription>Edit the commit message, then merge.</DialogDescription>
        </DialogHeader>
        <div className="flex flex-col gap-3">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="merge-subject">Commit message</Label>
            {/* A subject is one line, but a one-line *field* scrolls the end of
                it out of sight — the part that carries the PR number. This one
                wraps to show the whole thing (up to three lines) while keeping
                the value single-line: ⏎ merges instead of breaking, and a
                pasted newline collapses to a space. */}
            <textarea
              id="merge-subject"
              value={edit.subject}
              onChange={(e) => onChange({ ...edit, subject: e.target.value.replace(/\n/g, " ") })}
              onKeyDown={(e) => {
                if (e.key !== "Enter") return
                e.preventDefault()
                if (canMerge) onConfirm()
              }}
              autoFocus
              className={cn(commentFieldClass, "max-h-18 min-h-0 resize-none")}
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="merge-body">Extended description</Label>
            <textarea
              id="merge-body"
              value={edit.body}
              onChange={(e) => onChange({ ...edit, body: e.target.value })}
              placeholder="Optional"
              // The shared field grows with what it holds; the bounds are the
              // window's, as they are for a pull request's description — tall
              // enough to read the body without scrolling, short enough to
              // leave the merge button in sight.
              className={cn(commentFieldClass, "max-h-[50vh] min-h-[30vh]")}
            />
          </div>
        </div>
        <DialogFooter>
          <Button variant="ghost" onClick={onCancel}>
            Cancel
          </Button>
          <Button onClick={onConfirm} disabled={!canMerge}>
            <GitMerge />
            {merging ? "Merging…" : edit.method === "squash" ? "Squash and merge" : "Merge"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
