import { matchesCombo, matchesRepeatingCombo, type Hotkeys, type KeyState } from "@/lib/hotkeys"
import { isMac } from "@/lib/platform"

export type ZoomIntent = "in" | "out" | "reset"

export function zoomIntent(event: KeyState, hotkeys: Hotkeys, mac = isMac): ZoomIntent | null {
  if (matchesRepeatingCombo(event, hotkeys.zoomIn, mac)) return "in"
  if (matchesRepeatingCombo(event, hotkeys.zoomOut, mac)) return "out"
  if (matchesCombo(event, hotkeys.zoomReset, mac)) return "reset"
  return null
}
