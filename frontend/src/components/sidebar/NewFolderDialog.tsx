import { useEffect, useState } from "react"
import { Trans } from "@/components/common/Trans"
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
import { useT } from "@/lib/i18n/i18n"

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
  const t = useT()
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
          <DialogTitle>{t("sidebar.newFolderDialog.title")}</DialogTitle>
          <DialogDescription>
            {t("sidebar.newFolderDialog.description", { count })}
          </DialogDescription>
        </DialogHeader>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="new-folder-name">{t("sidebar.newFolderDialog.name")}</Label>
          <Input
            id="new-folder-name"
            value={value}
            onChange={(event) => setValue(event.target.value)}
            onKeyDown={(event) => event.key === "Enter" && commit()}
            placeholder={t("sidebar.newFolderDialog.placeholder")}
            autoFocus
          />
          {known && (
            <p className="text-xs text-muted-foreground">
              <Trans
                k="sidebar.newFolderDialog.exists"
                params={{ name: <span className="font-medium text-foreground">{name}</span> }}
              />
            </p>
          )}
        </div>
        <DialogFooter>
          <Button variant="ghost" onClick={() => onOpenChange(false)}>
            {t("common.action.cancel")}
          </Button>
          <Button onClick={commit} disabled={!name}>
            {known ? t("sidebar.newFolderDialog.moveHere") : t("common.action.create")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
