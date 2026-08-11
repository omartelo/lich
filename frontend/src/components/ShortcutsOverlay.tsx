import { useEffect, useMemo, useState } from "react"
import { Dialog as DialogPrimitive } from "@base-ui/react/dialog"
import { useSettings } from "@/providers/settings"
import { isRecordingTarget, matchesCombo } from "@/lib/hotkeys"
import { shortcutGroups } from "@/lib/shortcuts"

// Same signal HotkeysSettings and TerminalView read: the OS lich runs on.
const IS_MAC = navigator.platform.toLowerCase().includes("mac")
const IS_WINDOWS = navigator.platform.toLowerCase().includes("win")

// ShortcutsOverlay lists what is already bound and edits nothing — the one
// screen naming the shortcuts is Settings › Hotkeys, which is where a binding is
// changed, not where it is found. Mounted once at the app root; it renders
// nothing until opened.
//
// The trigger is caught in the window capture phase (like the palette) so it
// beats the shell chord it shadows; while open, focus is trapped in the dialog
// and keys never reach the terminal.
export function ShortcutsOverlay() {
  const { hotkeys } = useSettings()
  const [open, setOpen] = useState(false)

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (isRecordingTarget(event)) {
        return
      }
      if (matchesCombo(event, hotkeys.shortcuts)) {
        event.preventDefault()
        event.stopPropagation()
        setOpen((v) => !v)
      }
    }
    window.addEventListener("keydown", onKey, true)
    return () => window.removeEventListener("keydown", onKey, true)
  }, [hotkeys])

  const groups = useMemo(() => shortcutGroups(hotkeys, IS_MAC, IS_WINDOWS), [hotkeys])

  return (
    <DialogPrimitive.Root open={open} onOpenChange={setOpen}>
      <DialogPrimitive.Portal>
        <DialogPrimitive.Backdrop className="fixed inset-0 z-50 bg-black/50 supports-backdrop-filter:backdrop-blur-xs data-open:animate-in data-open:fade-in-0 data-closed:animate-out data-closed:fade-out-0" />
        <DialogPrimitive.Popup className="fixed left-1/2 top-[14vh] z-50 flex max-h-[70vh] w-full max-w-[34rem] -translate-x-1/2 flex-col overflow-hidden rounded-xl border bg-popover text-popover-foreground shadow-2xl ring-1 ring-foreground/10 outline-none data-open:animate-in data-open:fade-in-0 data-open:zoom-in-95 data-closed:animate-out data-closed:fade-out-0">
          <DialogPrimitive.Title className="border-b px-4 py-3 text-sm">
            Keyboard shortcuts
          </DialogPrimitive.Title>

          <div className="flex-1 overflow-y-auto p-1.5">
            {groups.map((group) => (
              <div key={group.title}>
                <div className="px-3 pb-1 pt-3 text-[0.65625rem] font-semibold uppercase tracking-wider text-muted-foreground">
                  {group.title}
                </div>
                {group.rows.map((row) => (
                  <div
                    key={row.label}
                    className="flex items-center gap-4 rounded-md px-3 py-2 hover:bg-accent/50"
                  >
                    <span className="flex-1 truncate text-sm">{row.label}</span>
                    <Keys>{row.keys}</Keys>
                  </div>
                ))}
              </div>
            ))}
          </div>

          <div className="flex items-center gap-4 border-t bg-black/10 px-4 py-2 text-xs text-muted-foreground">
            <span>Rebind in Settings › Hotkeys</span>
            <span className="ml-auto inline-flex items-center gap-1.5">
              <Keys>esc</Keys>
              close
            </span>
          </div>
        </DialogPrimitive.Popup>
      </DialogPrimitive.Portal>
    </DialogPrimitive.Root>
  )
}

function Keys({ children }: { children: React.ReactNode }) {
  return (
    <kbd className="shrink-0 rounded border border-b-2 bg-muted px-1.5 py-0.5 font-mono text-[0.625rem] leading-none text-muted-foreground">
      {children}
    </kbd>
  )
}
