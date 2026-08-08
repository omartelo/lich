import { SettingBlock } from "./SettingBlock"
import { Switch } from "@/components/ui/switch"
import { useSettings } from "@/providers/settings"

// The one setting that reaches outside the app window. An unanswered opt-in
// (null) reads as off here: until the user says yes, nothing notifies — and
// turning it on from this switch is itself an answer, so the dialog never
// appears afterwards.
export function NotificationsSettings() {
  const { desktopNotifications, setDesktopNotifications } = useSettings()
  return (
    <SettingBlock
      title="Notify me when a session needs input"
      description="Raise a desktop notification when a session is blocked on you — a permission prompt, a question — and the lich window is not the one you are in. While you are in lich nothing changes: the card's bell and the toast do the telling."
    >
      <Switch
        checked={desktopNotifications === true}
        onCheckedChange={setDesktopNotifications}
        aria-label="Raise a desktop notification when a session needs input"
      />
    </SettingBlock>
  )
}
