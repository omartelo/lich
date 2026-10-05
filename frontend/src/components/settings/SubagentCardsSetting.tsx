import { Switch } from "@/components/ui/switch"
import { subagentCardsKey, supportsSubagentCards } from "@/lib/providers-store"
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
  const [value, persist] = useStoredSetting(subagentCardsKey(providerId), GLOBAL_SCOPE)
  if (!supportsSubagentCards(providerId)) {
    return null
  }
  return (
    <SettingBlock
      title="Subagents as lich sessions"
      description={`A general-purpose subagent opens its own card and worktree instead of running hidden inside ${providerName}. Applies to sessions opened after you change it.`}
    >
      <Switch
        checked={value !== "false"}
        onCheckedChange={(on) => void persist(on ? "true" : "false")}
        aria-label={`Run ${providerName} subagents as lich sessions`}
      />
    </SettingBlock>
  )
}
