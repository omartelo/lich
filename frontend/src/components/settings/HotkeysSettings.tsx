import { useState } from "react"
import { Ban, RotateCcw, TriangleAlert } from "lucide-react"
import { Button } from "@/components/ui/button"
import {
  comboFromEvent,
  DEFAULT_HOTKEYS,
  formatCombo,
  hotkeyConflicts,
  hotkeyLabel,
  HOTKEY_ACTIONS,
  HOTKEY_GROUPS,
  sameCombo,
  UNASSIGNED,
  type HotkeyAction,
  type HotkeyId,
} from "@/lib/hotkeys"
import { isMac } from "@/lib/platform"
import { cn } from "@/lib/utils"
import { useSettings } from "@/providers/settings"

function Group({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <section className="pt-8 first:pt-0">
      <h2 className="mb-1 text-xs font-medium uppercase tracking-wide text-muted-foreground">
        {label}
      </h2>
      {children}
    </section>
  )
}

function HotkeyRow({ action, conflicts }: { action: HotkeyAction; conflicts?: HotkeyId[] }) {
  const { hotkeys, setHotkey, resetHotkey } = useSettings()
  const [recording, setRecording] = useState(false)
  const binding = hotkeys[action.id]
  const isDefault = sameCombo(binding, DEFAULT_HOTKEYS[action.id])

  const onKeyDown = (event: React.KeyboardEvent) => {
    if (!recording) return
    event.preventDefault()
    event.stopPropagation()
    if (event.key === "Escape") {
      setRecording(false)
      return
    }
    const next = comboFromEvent(event.nativeEvent, isMac)
    if (!next) return
    setHotkey(action.id, next)
    setRecording(false)
  }

  return (
    <div className="flex items-center gap-2 rounded-md px-2 py-1.5 hover:bg-accent/50">
      <div className="flex min-w-0 flex-1 flex-col">
        <span className="truncate text-sm text-foreground">{action.label}</span>
        {conflicts && (
          <span className="mt-0.5 inline-flex items-center gap-1 text-xs text-amber-500">
            <TriangleAlert className="size-3 shrink-0" />
            Also bound to {conflicts.map(hotkeyLabel).join(", ")}
          </span>
        )}
      </div>
      <button
        type="button"
        data-hotkey-capturing={recording ? "" : undefined}
        onClick={() => setRecording(true)}
        onKeyDown={onKeyDown}
        onBlur={() => setRecording(false)}
        className={cn(
          "min-w-36 shrink-0 rounded-md border px-3 py-1 text-left text-sm tabular-nums outline-none transition-colors focus-visible:ring-2 focus-visible:ring-ring",
          recording ? "border-ring text-muted-foreground" : "border-border hover:bg-accent",
          !recording && (binding ? "text-foreground" : "text-muted-foreground"),
          conflicts && !recording && "border-amber-500/60",
        )}
      >
        {recording ? "Press keys…" : formatCombo(binding, isMac)}
      </button>
      <Button
        variant="ghost"
        size="icon"
        aria-label={`Unassign ${action.label} shortcut`}
        title="Leave this action unbound"
        disabled={!binding}
        onClick={() => setHotkey(action.id, UNASSIGNED)}
      >
        <Ban />
      </Button>
      <Button
        variant="ghost"
        size="icon"
        aria-label={`Reset ${action.label} shortcut`}
        title="Restore the default shortcut"
        disabled={isDefault}
        onClick={() => resetHotkey(action.id)}
      >
        <RotateCcw />
      </Button>
    </div>
  )
}

export function HotkeysSettings() {
  const { hotkeys } = useSettings()
  const conflicts = hotkeyConflicts(hotkeys, isMac)

  return (
    <>
      <p className="mb-6 max-w-prose text-xs text-muted-foreground">
        Global shortcuts are captured by lich before they reach the terminal. Terminal translations
        are matched inside xterm and write substitute PTY sequences. Disabling stops that lich
        action or translation and restores native terminal behavior; dangerous Chromium accelerators
        stay guarded. Reset restores the platform default.
      </p>
      {HOTKEY_GROUPS.map((group) => (
        <Group key={group.id} label={group.label}>
          {HOTKEY_ACTIONS.filter((action) => action.group === group.id).map((action) => (
            <HotkeyRow key={action.id} action={action} conflicts={conflicts[action.id]} />
          ))}
        </Group>
      ))}
    </>
  )
}
