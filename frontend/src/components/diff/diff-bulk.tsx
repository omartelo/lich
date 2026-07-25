import { ChevronsDownUp, ChevronsUpDown } from "lucide-react"
import { useState } from "react"
import { IconAction } from "@/components/common/IconAction"

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
  return (
    <IconAction label={open ? "Collapse all files" : "Expand all files"} onClick={onToggle}>
      {open ? <ChevronsDownUp className="size-3.5" /> : <ChevronsUpDown className="size-3.5" />}
    </IconAction>
  )
}
