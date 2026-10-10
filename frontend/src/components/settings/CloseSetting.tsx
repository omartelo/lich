import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group"
import { CLOSE_ACTION_SETTING_KEY, CLOSE_ACTIONS, closeActionOf } from "@/lib/close-request"
import { useT } from "@/lib/i18n/i18n"
import { useStoredSetting } from "@/lib/use-stored-setting"
import { SettingBlock } from "./SettingBlock"

const GLOBAL_SCOPE = ""

// CloseSetting answers ahead of time what closing lich's window does, the
// question CloseDialog asks otherwise.
export function CloseSetting() {
  const t = useT()
  const [stored, persist] = useStoredSetting(CLOSE_ACTION_SETTING_KEY, GLOBAL_SCOPE)
  const choice = closeActionOf(stored)

  return (
    <SettingBlock
      title={t("settings.closeSetting.title")}
      description={t("settings.closeSetting.description")}
    >
      <ToggleGroup
        value={[choice]}
        // An empty array is the pressed choice pressed again: keep it.
        onValueChange={(next) => next[0] && void persist(next[0])}
        spacing={1}
        aria-label={t("settings.closeSetting.title")}
        className="border border-border p-[0.1875rem]"
      >
        {CLOSE_ACTIONS.map((c) => (
          <ToggleGroupItem key={c} value={c} size="sm">
            {t(`settings.closeSetting.choice.${c}.label`)}
          </ToggleGroupItem>
        ))}
      </ToggleGroup>
      <p className="mt-2 text-xs text-muted-foreground">
        {t(`settings.closeSetting.choice.${choice}.consequence`)}
      </p>
    </SettingBlock>
  )
}
