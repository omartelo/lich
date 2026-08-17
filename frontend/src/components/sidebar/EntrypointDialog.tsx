import { useEffect, useState } from "react"
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

interface EntrypointDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  // The command already on this session, "" for a terminal on a plain shell.
  entrypoint: string
  // Where the command will run, shown so a card in a worktree says which
  // checkout its lazygit is about to open.
  cwd: string
  onSave: (entrypoint: string) => void
}

// EntrypointDialog edits the one command a terminal session opens into. One
// field and no options: emptying it is how a card goes back to a plain shell,
// which is why there is no separate clear button to keep out of the footer.
export function EntrypointDialog({
  open,
  onOpenChange,
  entrypoint,
  cwd,
  onSave,
}: EntrypointDialogProps) {
  const [value, setValue] = useState(entrypoint)

  // Reseed on every open: the dialog outlives one editing session, and a card
  // reopened after a cancel would otherwise still hold the abandoned text.
  useEffect(() => {
    if (open) {
      setValue(entrypoint)
    }
  }, [open, entrypoint])

  const commit = () => {
    onOpenChange(false)
    const next = value.trim()
    if (next !== entrypoint) {
      onSave(next)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Entrypoint</DialogTitle>
          <DialogDescription>
            Runs each time this terminal starts. Quit it and you are back in the shell.
          </DialogDescription>
        </DialogHeader>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="session-entrypoint">Command</Label>
          <Input
            id="session-entrypoint"
            value={value}
            onChange={(event) => setValue(event.target.value)}
            onKeyDown={(event) => event.key === "Enter" && commit()}
            placeholder="lazygit"
            autoFocus
            className="font-mono"
          />
          <p className="text-xs text-muted-foreground">
            Runs in <span className="font-mono">{cwd}</span>. Leave empty for a plain shell.
          </p>
        </div>
        <DialogFooter>
          <Button variant="ghost" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button onClick={commit}>Save</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
