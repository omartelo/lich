// DiffStat is the +added/-deleted pair every diff surface shows (session card,
// footer, review panel, file header). One component so the palette stays
// consistent everywhere. It renders bare spans; the parent's flex gap spaces
// them.
export function DiffStat({ added, deleted }: { added: number; deleted: number }) {
  return (
    <>
      <span className="font-medium text-emerald-600 dark:text-emerald-400">+{added}</span>
      <span className="font-medium text-red-600 dark:text-red-400">-{deleted}</span>
    </>
  )
}
