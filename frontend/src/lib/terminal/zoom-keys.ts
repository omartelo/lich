import { matchesCombo, type Hotkeys, type KeyState } from "@/lib/hotkeys"
import { isMac } from "@/lib/platform"

export type ZoomIntent = "in" | "out" | "reset"

export function zoomIntent(event: KeyState, hotkeys: Hotkeys, mac = isMac): ZoomIntent | null {
  const repeating = { mac, allowRepeat: true }
  if (matchesCombo(event, hotkeys.zoomIn, repeating)) return "in"
  if (matchesCombo(event, hotkeys.zoomOut, repeating)) return "out"
  if (matchesCombo(event, hotkeys.zoomReset, { mac })) return "reset"
  return null
}
