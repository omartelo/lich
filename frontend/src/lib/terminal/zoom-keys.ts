import { matchesCombo, type Hotkeys, type KeyState } from "@/lib/hotkeys"
import { isMac } from "@/lib/platform"

export type ZoomIntent = "in" | "out" | "reset"
export type ZoomKeyState = KeyState

export function zoomIntent(event: ZoomKeyState, hotkeys: Hotkeys, mac = isMac): ZoomIntent | null {
  if (matchesCombo(event, hotkeys.zoomIn, mac)) return "in"
  if (matchesCombo(event, hotkeys.zoomOut, mac)) return "out"
  if (matchesCombo(event, hotkeys.zoomReset, mac)) return "reset"
  return null
}
