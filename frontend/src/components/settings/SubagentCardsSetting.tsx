import { Switch } from "@/components/ui/switch"
import { subagentCardsKey, supportsSubagentCards } from "@/lib/providers-store"
import { useT } from "@/lib/i18n/i18n"
import { useStoredSetting } from "@/lib/use-stored-setting"
import { SettingBlock } from "./SettingBlock"

const GLOBAL_SCOPE = ""

// SubagentCardsSetting runs a provider's general-purpose subagents as lich
// sessions. On until turned off, so only a stored "false" reads as off.
export function SubagentCardsSetting({
  providerId,
  providerName,
}: {
  providerId: string
  providerName: string
}) {
  const t = useT()
  const [value, persist] = useStoredSetting(subagentCardsKey(providerId), GLOBAL_SCOPE)
  if (!supportsSubagentCards(providerId)) {
    return null
  }
  return (
    <SettingBlock
      title={t("settings.subagentCardsSetting.title")}
      description={t("settings.subagentCardsSetting.description", { provider: providerName })}
    >
      <Switch
        checked={value !== "false"}
        onCheckedChange={(on) => void persist(on ? "true" : "false")}
        aria-label={t("settings.subagentCardsSetting.label", { provider: providerName })}
      />
    </SettingBlock>
  )
}
