import { Button } from "@/components/ui/button"
import { ConfirmDialog } from "@/components/ConfirmDialog"
import { Trans } from "@/components/common/Trans"
import type { DiffFile } from "@/lib/git/diff"
import { useT } from "@/lib/i18n/i18n"

interface DiscardDialogProps {
  /** The file whose changes are about to be reverted, or null when hidden. */
  file: DiffFile | null
  onCancel: () => void
  onDiscard: () => void
}

// DiscardDialog confirms reverting one file's uncommitted changes: a tracked
// file goes back to HEAD, a new file is deleted from disk. Either way the
// changes are gone for good.
export function DiscardDialog({ file, onCancel, onDiscard }: DiscardDialogProps) {
  const t = useT()
  return (
    <ConfirmDialog
      open={file !== null}
      onCancel={onCancel}
      title={t("diff.discardDialog.title")}
      description={
        <Trans
          k={
            file?.status === "added"
              ? "diff.discardDialog.descriptionAdded"
              : "diff.discardDialog.description"
          }
          params={{
            path: <span className="break-all font-mono select-text">{file?.newPath}</span>,
          }}
        />
      }
    >
      <Button variant="destructive" onClick={onDiscard}>
        {t("diff.discardDialog.confirm")}
      </Button>
    </ConfirmDialog>
  )
}
