import { useState } from "react"
import { RotateCcw } from "lucide-react"
import { useSettings } from "@/lib/settings"
import {
  comboFromEvent,
  DEFAULT_HOTKEYS,
  formatCombo,
  HOTKEY_ACTIONS,
  sameCombo,
  type HotkeyAction,
} from "@/lib/hotkeys"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"
import { SettingBlock } from "./SettingBlock"

const isMac = typeof navigator !== "undefined" && navigator.platform.toLowerCase().includes("mac")

// HotkeyRow shows the current combo as a capture button: click to record, press
// the new combo to save, Escape to cancel. Key events are swallowed while
// recording so they do not also trigger the very shortcut being rebound.
function HotkeyRow({ action }: { action: HotkeyAction }) {
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
    <div className="flex items-center gap-2">
      <button
        type="button"
        data-hotkey-capturing={recording ? "" : undefined}
        onClick={() => setRecording(true)}
        onKeyDown={onKeyDown}
        onBlur={() => setRecording(false)}
        className={cn(
          "min-w-40 rounded-md border px-3 py-1.5 text-left text-sm tabular-nums outline-none transition-colors focus-visible:ring-2 focus-visible:ring-ring",
          recording
            ? "border-ring text-muted-foreground"
            : "border-border text-foreground hover:bg-accent",
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
  return (
    <>
      {HOTKEY_ACTIONS.map((action) => (
        <SettingBlock key={action.id} title={action.label}>
          <HotkeyRow action={action} />
        </SettingBlock>
      ))}
    </>
  )
}
