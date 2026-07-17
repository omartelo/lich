import { setProviderEnabled, useProviders } from "@/lib/providers-store"
import { ProviderIcon } from "@/lib/provider-icons"
import { Switch } from "@/components/ui/switch"

// ProvidersSettings lists every known harness with its install state and an
// enable toggle. Enabling a provider surfaces its config section (a
// ProviderBinSettings entry) and offers it in New Session. A provider that is
// not installed cannot be turned on — but one already enabled (Claude, on by
// default) stays togglable so it is never trapped off-screen.
export function ProvidersSettings() {
  const providers = useProviders()

  if (providers.length === 0) {
    return (
      <p className="py-5 text-sm text-muted-foreground">Detecting providers…</p>
    )
  }

  return (
    <div className="flex flex-col divide-y divide-border">
      {providers.map((provider) => (
        <div key={provider.id} className="flex items-center justify-between gap-4 py-4">
          <div className="flex min-w-0 items-center gap-3">
            <ProviderIcon kind={provider.id} />
            <div className="min-w-0">
              <div className="text-sm font-medium text-foreground">{provider.name}</div>
              <div className="text-xs text-muted-foreground">
                {provider.installed ? "Installed" : "Not found on PATH"}
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
  )
}
