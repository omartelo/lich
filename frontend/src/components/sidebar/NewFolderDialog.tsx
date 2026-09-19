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

interface NewFolderDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  // How many sessions the name will file, so the dialog says what it is about
  // to do with a whole checkout's worth of cards.
  count: number
  // The folders the project already holds: a name already among them files into
  // that folder rather than making a second one, which is what the dialog says
  // before it happens.
  existing: string[]
  onCreate: (name: string) => void
}

// NewFolderDialog names a folder. One field, because a folder is only a name:
// filing the first session under it is what creates it, and taking the last one
// out is what ends it — there is nothing else to configure and nothing to
// delete afterwards.
export function NewFolderDialog({
  open,
  onOpenChange,
  count,
  existing,
  onCreate,
}: NewFolderDialogProps) {
  const [value, setValue] = useState("")

  // Reseed on every open: the dialog outlives one naming, and a second card sent
  // here would otherwise arrive holding the abandoned text.
  useEffect(() => {
    if (open) {
      setValue("")
    }
  }, [open])

  const name = value.trim()
  const known = existing.includes(name)

  const commit = () => {
    if (!name) {
      return
    }
    onOpenChange(false)
    onCreate(name)
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>New folder</DialogTitle>
          <DialogDescription>
            {count === 1
              ? "Files this session under a name of your own. It stays open, and its checkout does not change."
              : `Files these ${count} sessions under a name of your own. They stay open, and their checkouts do not change.`}
          </DialogDescription>
        </DialogHeader>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="new-folder-name">Name</Label>
          <Input
            id="new-folder-name"
            value={value}
            onChange={(event) => setValue(event.target.value)}
            onKeyDown={(event) => event.key === "Enter" && commit()}
            placeholder="Design system"
            autoFocus
          />
          {known && (
            <p className="text-xs text-muted-foreground">
              <span className="font-medium text-foreground">{name}</span> already exists — this
              files into it.
            </p>
          )}
        </div>
        <DialogFooter>
          <Button variant="ghost" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button onClick={commit} disabled={!name}>
            {known ? "Move here" : "Create"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
