import { useState } from "react"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"

/** Where a comment lands: the file, and the new-file lines the selection covered. */
export interface CommentAnchor {
  path: string
  lines: string
}

interface CommentDialogProps {
  /** The lines being commented on, or null while the dialog is closed. */
  anchor: CommentAnchor | null
  onCancel: () => void
  onAdd: (text: string) => void
}

// CommentDialog collects one line comment — the note the user would otherwise
// have typed into the terminal while looking at these lines. Text only: where
// the comment goes, and when the batch is sent, stay the panel's.
export function CommentDialog({ anchor, onCancel, onAdd }: CommentDialogProps) {
  const [text, setText] = useState("")

  const dismiss = () => {
    setText("")
    onCancel()
  }

  const add = () => {
    if (text.trim() === "") {
      return
    }
    onAdd(text)
    setText("")
  }

  return (
    <Dialog open={anchor !== null} onOpenChange={(next) => !next && dismiss()}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Comment</DialogTitle>
          <DialogDescription className="break-all font-mono">
            {anchor && `${anchor.path}:${anchor.lines}`}
          </DialogDescription>
        </DialogHeader>
        <textarea
          aria-label="Comment"
          value={text}
          onChange={(e) => setText(e.target.value)}
          // Enter belongs to the comment — a review note runs to several lines
          // often enough — so the modifier chord is what adds it.
          onKeyDown={(e) => {
            if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
              e.preventDefault()
              add()
            }
          }}
          rows={4}
          autoFocus
          placeholder="What should change here?"
          className="min-h-24 w-full resize-y rounded-md border border-input bg-transparent px-2.5 py-1.5 text-sm shadow-xs outline-none transition-[color,box-shadow] placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 dark:bg-input/30"
        />
        <DialogFooter>
          <Button variant="ghost" onClick={dismiss}>
            Cancel
          </Button>
          <Button onClick={add} disabled={text.trim() === ""}>
            Add comment
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
