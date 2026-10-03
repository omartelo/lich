import { Switch } from "@/components/ui/switch"
import { supportsUltracode, ultracodeKey } from "@/lib/providers-store"
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
  const [on, setOn] = useStoredFlag(ultracodeKey(providerId), GLOBAL_SCOPE)
  if (!supportsUltracode(providerId)) {
    return null
  }
  return (
    <SettingBlock
      title="Ultracode"
      description={`Every ${providerName} session starts with ultracode on, orchestrating multi-agent workflows at whatever effort it runs. It stays on when a session restarts or resumes.`}
    >
      <Switch
        checked={on}
        onCheckedChange={setOn}
        aria-label={`Start ${providerName} sessions in ultracode`}
      />
    </SettingBlock>
  )
}
