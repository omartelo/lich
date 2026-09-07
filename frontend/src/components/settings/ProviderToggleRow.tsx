import { ProviderIcon } from "@/components/ProviderIcon"
import { Switch } from "@/components/ui/switch"
import { setProviderEnabled, type ProviderState } from "@/lib/providers-store"
import { ProviderDocsLink } from "./ProviderDocsLink"

// One provider offered for turning on or off: what it is, whether the machine
// has it, and the switch. Shared by the first-run dialog and the Providers
// pane's "Add provider" list so the two cannot drift on what a provider lich
// could not find looks like.
//
// A provider that is not installed cannot be turned on, but one already
// enabled stays togglable, so it is never trapped on by a binary that moved.
export function ProviderToggleRow({ provider }: { provider: ProviderState }) {
  return (
    <div className="flex items-center justify-between gap-4 py-3">
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
  )
}
