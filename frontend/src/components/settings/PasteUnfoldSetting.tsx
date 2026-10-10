import { Switch } from "@/components/ui/switch"
import { pasteUnfoldKey, pasteUnfoldTiming } from "@/lib/providers-store"
import { useT } from "@/lib/i18n/i18n"
import { useStoredFlag } from "@/lib/use-stored-setting"
import { SettingBlock } from "./SettingBlock"

const GLOBAL_SCOPE = ""

// PasteUnfoldSetting makes a long paste land as the full text in a provider that
// would fold it into a placeholder. Absent for a provider whose fold lich cannot
// undo, where the switch would store a setting nothing reads.
export function PasteUnfoldSetting({
  providerId,
  providerName,
}: {
  providerId: string
  providerName: string
}) {
  const t = useT()
  const [on, setOn] = useStoredFlag(pasteUnfoldKey(providerId), GLOBAL_SCOPE)
  const timing = pasteUnfoldTiming(providerId)
  if (timing === null) {
    return null
  }
  const description =
    timing === "nextPaste"
      ? t("settings.pasteUnfoldSetting.descriptionNextPaste", { provider: providerName })
      : t("settings.pasteUnfoldSetting.descriptionNextSession", { provider: providerName })
  return (
    <SettingBlock title={t("settings.pasteUnfoldSetting.title")} description={description}>
      <Switch
        checked={on}
        onCheckedChange={setOn}
        aria-label={t("settings.pasteUnfoldSetting.label", { provider: providerName })}
      />
    </SettingBlock>
  )
}
