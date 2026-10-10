import { useEffect, useState } from "react"
import { Download, FileJson } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Trans } from "@/components/common/Trans"
import { useT } from "@/lib/i18n/i18n"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"

interface ImportThemeDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** Clone and install a repository. Resolves once the attempt is finished. */
  onInstallRepository: (url: string) => Promise<void>
  /** Open the native picker for a single theme file. */
  onChooseFile: () => Promise<void>
  /** Save the theme template, the starting point for writing one. */
  onDownloadTemplate: () => void
  /** True while a clone or a file import is in flight. */
  busy: boolean
}

// The two ways a theme arrives, in one place: a repository lich can clone
// again later, and the single file that has always worked and stays unversioned.
export function ImportThemeDialog({
  open,
  onOpenChange,
  onInstallRepository,
  onChooseFile,
  onDownloadTemplate,
  busy,
}: ImportThemeDialogProps) {
  const t = useT()
  const [url, setUrl] = useState("")

  useEffect(() => {
    if (!open) setUrl("")
  }, [open])

  const trimmed = url.trim()

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{t("settings.importThemeDialog.title")}</DialogTitle>
          <DialogDescription>{t("settings.importThemeDialog.description")}</DialogDescription>
        </DialogHeader>
        <form
          className="flex flex-col gap-2"
          onSubmit={(event) => {
            event.preventDefault()
            if (trimmed && !busy) void onInstallRepository(trimmed)
          }}
        >
          <Label htmlFor="theme-repository-url">
            {t("settings.importThemeDialog.repositoryUrl")}
          </Label>
          <div className="flex items-center gap-2">
            <Input
              id="theme-repository-url"
              className="font-mono text-xs"
              value={url}
              onChange={(event) => setUrl(event.target.value)}
              placeholder="https://github.com/you/lich-themes.git"
              disabled={busy}
            />
            <Button type="submit" disabled={!trimmed || busy}>
              {busy
                ? t("settings.importThemeDialog.installing")
                : t("settings.importThemeDialog.install")}
            </Button>
          </div>
          <p className="text-xs text-muted-foreground">
            <Trans
              k="settings.importThemeDialog.repositoryHint"
              params={{ file: <code className="font-mono">lich-theme.json</code> }}
            />
          </p>
        </form>
        <div className="flex items-center gap-3 text-xs text-muted-foreground">
          <span className="h-px flex-1 bg-border" />
          {t("settings.importThemeDialog.or")}
          <span className="h-px flex-1 bg-border" />
        </div>
        <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
          <Button
            type="button"
            variant="outline"
            disabled={busy}
            onClick={() => void onChooseFile()}
          >
            <FileJson />
            {t("settings.importThemeDialog.chooseFile")}
          </Button>
          <span className="text-xs text-muted-foreground">
            {t("settings.importThemeDialog.fileHint")}
          </span>
        </div>
        <DialogFooter className="sm:justify-between">
          {/* Writing a theme starts here, so the template belongs to this
              dialog. On the pane it was an unlabelled download glyph beside
              Import, which named neither the file nor what it was for. */}
          <Button type="button" variant="ghost" disabled={busy} onClick={onDownloadTemplate}>
            <Download />
            {t("settings.importThemeDialog.downloadTemplate")}
          </Button>
          <Button variant="ghost" disabled={busy} onClick={() => onOpenChange(false)}>
            {t("common.action.cancel")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
