import { useState } from "react"
import { RotateCcw, TriangleAlert } from "lucide-react"
import { useSettings } from "@/providers/settings"
import {
  comboFromEvent,
  DEFAULT_HOTKEYS,
  formatCombo,
  hotkeyConflicts,
  HOTKEY_ACTIONS,
  HOTKEY_GROUPS,
  sameCombo,
  type HotkeyAction,
  type HotkeyId,
} from "@/lib/hotkeys"
import { PASSTHROUGH_TITLE, passthroughRows } from "@/lib/shortcuts"
import { Button } from "@/components/ui/button"
import { Keys } from "@/components/common/Keys"
import { cn } from "@/lib/utils"

const platform = typeof navigator === "undefined" ? "" : navigator.platform.toLowerCase()
const isMac = platform.includes("mac")
const isWindows = platform.includes("win")

const LABELS = Object.fromEntries(HOTKEY_ACTIONS.map((a) => [a.id, a.label])) as Record<
  HotkeyId,
  string
>

// A dense list rather than one setting block per action: eleven bindings read as
// a table of rows, and a block each would be a column of near-empty cards.
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

// HotkeyRow shows the current combo as a capture button: click to record, press
// the new combo to save, Escape to cancel. Key events are swallowed while
// recording so they do not also trigger the very shortcut being rebound.
function HotkeyRow({ action, conflicts }: { action: HotkeyAction; conflicts?: HotkeyId[] }) {
  const { hotkeys, setHotkey, resetHotkey } = useSettings()
  const [recording, setRecording] = useState(false)
  const combo = hotkeys[action.id]
  const isDefault = sameCombo(combo, DEFAULT_HOTKEYS[action.id])

  const onKeyDown = (event: React.KeyboardEvent) => {
    if (!recording) return
    event.preventDefault()
    event.stopPropagation()
    if (event.key === "Escape") {
      setRecording(false)
      return
    }
    const next = comboFromEvent(event.nativeEvent)
    if (next) {
      setHotkey(action.id, next)
      setRecording(false)
    }
  }

  return (
    <div className="flex items-center gap-4 rounded-md px-2 py-1.5 hover:bg-accent/50">
      <div className="flex min-w-0 flex-1 flex-col">
        <span className="truncate text-sm text-foreground">{action.label}</span>
        {conflicts && (
          // Amber because it is a state, not a failure: the binding was stored,
          // and whichever listener runs first is the one that will answer to it.
          <span className="mt-0.5 inline-flex items-center gap-1 text-xs text-amber-500">
            <TriangleAlert className="size-3 shrink-0" />
            Also bound to {conflicts.map((id) => LABELS[id]).join(", ")}
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
          recording
            ? "border-ring text-muted-foreground"
            : "border-border text-foreground hover:bg-accent",
          conflicts && !recording && "border-amber-500/60",
        )}
      >
        {recording ? "Press keys…" : formatCombo(combo, isMac)}
      </button>
      <Button
        variant="ghost"
        size="icon"
        aria-label={`Reset ${action.label} shortcut`}
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
  const conflicts = hotkeyConflicts(hotkeys)

  return (
    <>
      {HOTKEY_GROUPS.map((group) => (
        <Group key={group.id} label={group.label}>
          {HOTKEY_ACTIONS.filter((action) => action.group === group.id).map((action) => (
            <HotkeyRow key={action.id} action={action} conflicts={conflicts[action.id]} />
          ))}
        </Group>
      ))}
      <Group label={PASSTHROUGH_TITLE}>
        <p className="mb-2 max-w-prose text-xs text-muted-foreground">
          lich rewrites these on their way to the agent's terminal, so they are fixed rather than
          rebindable.
        </p>
        {passthroughRows(isWindows).map((row) => (
          <div
            key={row.label}
            className="flex items-center gap-4 rounded-md px-2 py-1.5 hover:bg-accent/50"
          >
            <span className="min-w-0 flex-1 truncate text-sm text-foreground">{row.label}</span>
            <Keys>{row.keys}</Keys>
          </div>
        ))}
      </Group>
    </>
  )
}
