import { ChevronsDownUp, ChevronsUpDown, Space } from "lucide-react"
import { useState } from "react"
import { IconAction } from "@/components/common/IconAction"
import { useT } from "@/lib/i18n/i18n"
import { useSettings } from "@/providers/settings"

// A collapse/expand-all directive shared by every file in a panel. The nonce is
// bumped on each bulk action so files re-sync even to a target they already
// hold — without it, a file left open by hand would ignore "expand all".
export interface DiffBulk {
  open: boolean
  nonce: number
}

// useDiffBulk owns that directive for one panel. Files start expanded, subject
// to each file's own large-file default until the first bulk action fires.
export function useDiffBulk(): [DiffBulk, () => void] {
  const [bulk, setBulk] = useState<DiffBulk>({ open: true, nonce: 0 })
  return [bulk, () => setBulk((b) => ({ open: !b.open, nonce: b.nonce + 1 }))]
}

// The header control driving it, in both panels that hold a list of diffs: the
// review dock and a pull request's Files changed tab.
export function CollapseAllAction({ open, onToggle }: { open: boolean; onToggle: () => void }) {
  const t = useT()
  return (
    <IconAction
      label={open ? t("diff.diffBulk.collapseAll") : t("diff.diffBulk.expandAll")}
      onClick={onToggle}
    >
      {open ? <ChevronsDownUp className="size-3.5" /> : <ChevronsUpDown className="size-3.5" />}
    </IconAction>
  )
}

// Beside it, and in the same two panels: one preference for every diff, so the
// dock and a pull request never disagree about what a change is.
export function HideWhitespaceAction() {
  const t = useT()
  const { hideWhitespace, setHideWhitespace } = useSettings()
  return (
    <IconAction
      label={hideWhitespace ? t("diff.diffBulk.showWhitespace") : t("diff.diffBulk.hideWhitespace")}
      onClick={() => setHideWhitespace(!hideWhitespace)}
      pressed={hideWhitespace}
    >
      <Space className="size-3.5" />
    </IconAction>
  )
}
