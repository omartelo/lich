import { Keys } from "@/components/common/Keys"

// One listed shortcut: what it does on the left, the chord on the right. Shared
// by the shortcuts overlay and the read-only half of Settings › Hotkeys so the
// same row cannot drift into two shapes.
export function ShortcutLine({ label, keys }: { label: string; keys: string }) {
  return (
    <div className="flex items-center gap-4 rounded-md px-3 py-1.5 hover:bg-accent/50">
      <span className="min-w-0 flex-1 truncate text-sm">{label}</span>
      <Keys>{keys}</Keys>
    </div>
  )
}
