import {
  enabledProviders,
  setProviderDefault,
  useDefaultProvider,
  useProviders,
} from "@/lib/providers-store"
import { ProviderRefreshButton } from "./ProviderRefreshButton"
import { ProviderSelect } from "./ProviderSelect"
import { ProviderToggleRow } from "./ProviderToggleRow"
import { SettingBlock } from "./SettingBlock"

// ProvidersSettings is the first-run question: which of the agents installed on
// this machine should lich offer, and which one do new sessions spawn. Enabling
// a provider offers it in New Session and gives it a row in Settings ›
// Providers, where its binary and permission rung live.
//
// Narrowed to what is on PATH, unlike the roster in Settings: here the question
// is which of *your* agents to use, so rows for agents the machine does not have
// are noise. There they are the useful state, next to the docs link that says
// how to install one.
export function ProvidersSettings() {
  const providers = useProviders()
  const defaultProvider = useDefaultProvider()

  if (providers.length === 0) {
    return <p className="py-5 text-sm text-muted-foreground">Detecting providers…</p>
  }

  const listed = providers.filter((provider) => provider.installed)
  const enabled = enabledProviders(listed)

  return (
    <div className="flex flex-col">
      {enabled.length > 0 && (
        <SettingBlock
          title="Default provider"
          description="The provider implicit session actions use, unless the project you are in picks its own in Settings › Providers."
        >
          <ProviderSelect
            providers={enabled}
            value={defaultProvider}
            ariaLabel="Global default provider"
            onChange={setProviderDefault}
          />
        </SettingBlock>
      )}
      <div className="flex items-center justify-between pb-1.5 pt-4">
        <h2 className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
          Detected on PATH
        </h2>
        <ProviderRefreshButton />
      </div>
      <div className="flex flex-col divide-y divide-border">
        {listed.map((provider) => (
          <ProviderToggleRow key={provider.id} provider={provider} />
        ))}
      </div>
    </div>
  )
}
