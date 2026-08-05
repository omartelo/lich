import { useEffect, useState } from "react"
import { FileJson } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
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
  busy,
}: ImportThemeDialogProps) {
  const [url, setUrl] = useState("")

  useEffect(() => {
    if (!open) setUrl("")
  }, [open])

  const trimmed = url.trim()

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Import theme</DialogTitle>
          <DialogDescription>
            Install from a git repository to keep the theme versioned, or pick a single theme file.
          </DialogDescription>
        </DialogHeader>
        <form
          className="flex flex-col gap-2"
          onSubmit={(event) => {
            event.preventDefault()
            if (trimmed && !busy) void onInstallRepository(trimmed)
          }}
        >
          <Label htmlFor="theme-repository-url">Repository URL</Label>
          <div className="flex items-center gap-2">
            <Input
              id="theme-repository-url"
              className="font-mono text-xs"
              value={url}
              onChange={(event) => setUrl(event.target.value)}
              placeholder="https://github.com/you/lich-themes.git"
              autoComplete="off"
              spellCheck={false}
              disabled={busy}
            />
            <Button type="submit" disabled={!trimmed || busy}>
              {busy ? "Installing…" : "Install"}
            </Button>
          </div>
          <p className="text-xs text-muted-foreground">
            A repository holding <code className="font-mono">lich-theme.json</code> and one or more
            theme files. Its manifest version is what lich compares on update.
          </p>
        </form>
        <div className="flex items-center gap-3 text-xs text-muted-foreground">
          <span className="h-px flex-1 bg-border" />
          or
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
            Choose file…
          </Button>
          <span className="text-xs text-muted-foreground">
            A single theme file. No version, no updates.
          </span>
        </div>
        <DialogFooter>
          <Button variant="ghost" disabled={busy} onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
