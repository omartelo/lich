import { Switch } from "@/components/ui/switch"
import { supportsUltracode, ultracodeKey } from "@/lib/providers-store"
import { useT } from "@/lib/i18n/i18n"
import { useStoredFlag } from "@/lib/use-stored-setting"
import { SettingBlock } from "./SettingBlock"

const GLOBAL_SCOPE = ""

// UltracodeSetting starts every session of a provider with ultracode on. Absent
// for a provider without one, where the switch would store a setting nothing
// reads.
export function UltracodeSetting({
  providerId,
  providerName,
}: {
  providerId: string
  providerName: string
}) {
  const t = useT()
  const [on, setOn] = useStoredFlag(ultracodeKey(providerId), GLOBAL_SCOPE)
  if (!supportsUltracode(providerId)) {
    return null
  }
  return (
    <SettingBlock
      title={t("settings.ultracodeSetting.title")}
      description={t("settings.ultracodeSetting.description", { provider: providerName })}
    >
      <Switch
        checked={on}
        onCheckedChange={setOn}
        aria-label={t("settings.ultracodeSetting.label", { provider: providerName })}
      />
    </SettingBlock>
  )
}
