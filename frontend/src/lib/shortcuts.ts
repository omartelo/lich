import { formatCombo, HOTKEY_ACTIONS, HOTKEY_GROUPS, type Hotkeys } from "@/lib/hotkeys"

export interface ShortcutRow {
  label: string
  keys: string
}

export interface ShortcutGroup {
  title: string
  rows: ShortcutRow[]
}

export function shortcutGroups(hotkeys: Hotkeys, isMac: boolean): ShortcutGroup[] {
  return HOTKEY_GROUPS.map((group) => ({
    title: group.label,
    rows: HOTKEY_ACTIONS.filter((action) => action.group === group.id).map((action) => ({
      label: action.label,
      keys: formatCombo(hotkeys[action.id], isMac),
    })),
  }))
}
