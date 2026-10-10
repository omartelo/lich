import { SettingBlock } from "./SettingBlock"
import { Switch } from "@/components/ui/switch"
import { useT } from "@/lib/i18n/i18n"
import { useSettings } from "@/providers/settings"

// The two settings that reach outside the app window, one per reason to
// interrupt. An unanswered opt-in (null) reads as off for the first: until the
// user says yes, nothing notifies — and turning it on from this switch is itself
// an answer, so the dialog never appears afterwards. The second is never asked
// about at all; it is off until it is turned on here.
export function NotificationsSettings() {
  const t = useT()
  const {
    desktopNotifications,
    setDesktopNotifications,
    finishedTurnNotifications,
    setFinishedTurnNotifications,
  } = useSettings()
  return (
    <>
      <SettingBlock
        title={t("settings.notificationsSettings.needsInputTitle")}
        description={t("settings.notificationsSettings.needsInputDescription")}
      >
        <Switch
          checked={desktopNotifications === true}
          onCheckedChange={setDesktopNotifications}
          aria-label={t("settings.notificationsSettings.needsInputLabel")}
        />
      </SettingBlock>

      <SettingBlock
        title={t("settings.notificationsSettings.finishedTitle")}
        description={t("settings.notificationsSettings.finishedDescription")}
      >
        <Switch
          checked={finishedTurnNotifications}
          onCheckedChange={setFinishedTurnNotifications}
          aria-label={t("settings.notificationsSettings.finishedLabel")}
        />
      </SettingBlock>
    </>
  )
}
