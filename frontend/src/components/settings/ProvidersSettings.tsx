import {
  enabledProviders,
  setProviderDefault,
  setProviderEnabled,
  useDefaultProvider,
  useProviders,
} from "@/lib/providers-store"
import { ProviderIcon } from "@/components/ProviderIcon"
import { ProviderDocsLink } from "./ProviderDocsLink"
import { ProviderRefreshButton } from "./ProviderRefreshButton"
import { SettingBlock } from "./SettingBlock"
import { Switch } from "@/components/ui/switch"
import { ProviderSelect } from "./ProviderSelect"

interface ProvidersSettingsProps {
  installedOnly?: boolean
}

// ProvidersSettings lists every known provider with its install state and an
// enable toggle, and picks the one new sessions spawn by default. Enabling a
// provider surfaces its config section (a ProviderBinSettings entry) and offers
// it in New Session. A provider that is not installed cannot be turned on — but
// one already enabled (Claude, on by default) stays togglable so it is never
// trapped off-screen.
//
// installedOnly narrows the list to what is on PATH, for the first-run dialog:
// there the question is which of your agents to use, so "Not found on PATH" rows
// are noise. In Settings they are the useful state and the full roster shows.
export function ProvidersSettings({ installedOnly = false }: ProvidersSettingsProps) {
  const providers = useProviders()
  const defaultProvider = useDefaultProvider()

  if (providers.length === 0) {
    return <p className="py-5 text-sm text-muted-foreground">Detecting providers…</p>
  }

  const listed = installedOnly ? providers.filter((provider) => provider.installed) : providers
  const enabled = enabledProviders(listed)

  return (
    <div className="flex flex-col">
      {enabled.length > 0 && (
        <SettingBlock
          title="Default provider"
          description="The provider implicit session actions use unless the current project has its own choice in Project Settings › Providers."
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
          <div key={provider.id} className="flex items-center justify-between gap-4 py-4">
            <div className="flex min-w-0 items-center gap-3">
              <ProviderIcon kind={provider.id} />
              <div className="min-w-0">
                <div className="text-sm font-medium text-foreground">{provider.name}</div>
                <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
                  {provider.installed ? (
                    "Installed"
                  ) : (
                    <>
                      <span>Not found on PATH</span>
                      <span aria-hidden>·</span>
                      <ProviderDocsLink provider={provider} />
                    </>
                  )}
                </div>
              </div>
            </div>
            <Switch
              checked={provider.enabled}
              disabled={!provider.installed && !provider.enabled}
              onCheckedChange={(checked) => setProviderEnabled(provider.id, checked)}
              aria-label={`Enable ${provider.name}`}
            />
          </div>
        ))}
      </div>
    </div>
  )
}
