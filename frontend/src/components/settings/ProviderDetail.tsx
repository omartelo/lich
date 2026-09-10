import { ChevronLeft } from "lucide-react"
import { ProviderIcon } from "@/components/ProviderIcon"
import { Switch } from "@/components/ui/switch"
import { countOpenSessions } from "@/lib/provider-summary"
import { setProviderEnabled, type ProviderState } from "@/lib/providers-store"
import { useProjects } from "@/providers/projects"
import { ProviderBinSettings } from "./ProviderBinSettings"
import { ProviderDocsLink } from "./ProviderDocsLink"
import { SettingBlock } from "./SettingBlock"

// ProviderDetail is one provider's own screen, reached from the Providers list.
// It is a screen rather than a drawer inside the list because the blocks it
// holds are the same shape as every other settings block (title, description,
// control) and those do not survive being squeezed into a row: the description
// under "Skip permission prompts" is the sentence that says what handing the
// machine over means, and it was the first thing a drawer dropped.
export function ProviderDetail({
  provider,
  projectId,
  onBack,
}: {
  provider: ProviderState
  projectId?: string
  onBack: () => void
}) {
  const { sessions } = useProjects()
  const open = countOpenSessions(sessions, provider.id)

  return (
    <>
      <button
        type="button"
        onClick={onBack}
        className="mb-1 inline-flex items-center gap-1 rounded-md text-xs text-muted-foreground transition-colors hover:text-foreground"
      >
        <ChevronLeft className="size-3.5" aria-hidden="true" />
        All providers
      </button>
      <div className="mb-4 flex items-center gap-3">
        <ProviderIcon kind={provider.id} size={22} />
        <h1 className="text-2xl font-semibold text-foreground">{provider.name}</h1>
        <div className="ml-auto flex items-center gap-2">
          <span className="text-xs text-muted-foreground">Enabled</span>
          <Switch
            checked={provider.enabled}
            onCheckedChange={(checked) => setProviderEnabled(provider.id, checked)}
            aria-label={`Enable ${provider.name}`}
          />
        </div>
      </div>

      <div className="divide-y divide-border">
        <ProviderBinSettings
          providerId={provider.id}
          providerName={provider.name}
          providerBin={provider.binary}
          projectId={projectId}
        />
        {/* What is true of this provider outside its settings, and the only
            answer the switch above owes: turning it off with sessions running
            leaves those sessions alone, and the count is how you know there
            are any. */}
        <SettingBlock title="Right now">
          <div className="flex flex-wrap items-center gap-4 text-xs text-muted-foreground">
            <span>
              {open === 0 ? "No sessions open" : `${open} session${open === 1 ? "" : "s"} open`}
            </span>
            <ProviderDocsLink provider={provider} label="Documentation" />
          </div>
        </SettingBlock>
      </div>
    </>
  )
}
