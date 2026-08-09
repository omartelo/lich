import { SettingBlock } from "./SettingBlock"
import { Switch } from "@/components/ui/switch"
import { useSettings } from "@/providers/settings"

// The two settings that reach outside the app window, one per reason to
// interrupt. An unanswered opt-in (null) reads as off for the first: until the
// user says yes, nothing notifies — and turning it on from this switch is itself
// an answer, so the dialog never appears afterwards. The second is never asked
// about at all; it is off until it is turned on here.
export function NotificationsSettings() {
  const {
    desktopNotifications,
    setDesktopNotifications,
    finishedTurnNotifications,
    setFinishedTurnNotifications,
  } = useSettings()
  return (
    <>
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

      <SettingBlock
        title="Notify me when a session finishes working"
        description="Raise a desktop notification when a session ends its turn with nothing left to ask you, and the lich window is not the one you are in. Independent of the setting above — a long run you walked away from is worth knowing about even if you would rather not hear about every prompt. No toast comes with it: inside lich, the card and its project tab already say so."
      >
        <Switch
          checked={finishedTurnNotifications}
          onCheckedChange={setFinishedTurnNotifications}
          aria-label="Raise a desktop notification when a session finishes working"
        />
      </SettingBlock>
    </>
  )
}
