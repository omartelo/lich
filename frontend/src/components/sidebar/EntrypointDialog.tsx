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
  const t = useT()
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
          <DialogTitle>{t("sidebar.entrypointDialog.title")}</DialogTitle>
          <DialogDescription>{t("sidebar.entrypointDialog.description")}</DialogDescription>
        </DialogHeader>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="session-entrypoint">{t("sidebar.entrypointDialog.command")}</Label>
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
            <Trans
              k="sidebar.entrypointDialog.hint"
              params={{ cwd: <span className="font-mono select-text">{cwd}</span> }}
            />
          </p>
        </div>
        <DialogFooter>
          <Button variant="ghost" onClick={() => onOpenChange(false)}>
            {t("common.action.cancel")}
          </Button>
          <Button onClick={commit}>{t("common.action.save")}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
