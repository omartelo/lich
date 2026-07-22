import {
  enabledProviders,
  setProviderDefault,
  setProviderEnabled,
  useDefaultProvider,
  useProviders,
} from "@/lib/providers-store"
import { ProviderIcon } from "@/lib/provider-icons"
import type { ProviderKind } from "@/lib/sessions"
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { SettingBlock } from "./SettingBlock"
import { Switch } from "@/components/ui/switch"

// ProvidersSettings lists every known harness with its install state and an
// enable toggle, and picks the one new sessions spawn by default. Enabling a
// provider surfaces its config section (a ProviderBinSettings entry) and offers
// it in New Session. A provider that is not installed cannot be turned on — but
// one already enabled (Claude, on by default) stays togglable so it is never
// trapped off-screen.
export function ProvidersSettings() {
  const providers = useProviders()
  const defaultProvider = useDefaultProvider()

  if (providers.length === 0) {
    return (
      <p className="py-5 text-sm text-muted-foreground">Detecting providers…</p>
    )
  }

  const enabled = enabledProviders(providers)

  return (
    <div className="flex flex-col">
      {enabled.length > 0 && (
        <SettingBlock
          title="Default provider"
          description="Which harness a new worktree, the new-session hotkey, and a project's first session spawn."
        >
          <Select
            value={defaultProvider}
            onValueChange={(value) => value && setProviderDefault(value as ProviderKind)}
          >
            <SelectTrigger className="w-64">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectGroup>
                {enabled.map((provider) => (
                  <SelectItem key={provider.id} value={provider.id}>
                    <span className="flex items-center gap-2">
                      <ProviderIcon kind={provider.id} size={14} />
                      {provider.name}
                    </span>
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
        </SettingBlock>
      )}
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
    </div>
  )
}
