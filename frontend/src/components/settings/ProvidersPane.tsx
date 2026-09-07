import { useState } from "react"
import { ChevronDown, ChevronRight, Plus, X } from "lucide-react"
import { ProviderIcon } from "@/components/ProviderIcon"
import { Button } from "@/components/ui/button"
import { Switch } from "@/components/ui/switch"
import { planSummary } from "@/lib/provider-summary"
import {
  enabledProviders,
  setProjectProviderDefault,
  setProviderDefault,
  setProviderEnabled,
  skipLevel,
  skipPermissionsKey,
  useDefaultProvider,
  useProviders,
  useStoredProjectDefaultProvider,
  type ProviderState,
} from "@/lib/providers-store"
import { usePlanQuotaFor } from "@/lib/quota/use-plan-quota"
import { useStoredFlag } from "@/lib/use-stored-setting"
import { useProjects } from "@/providers/projects"
import { cn } from "@/lib/utils"
import { ProviderDetail } from "./ProviderDetail"
import { ProviderRefreshButton } from "./ProviderRefreshButton"
import { ProviderSelect } from "./ProviderSelect"
import { ProviderToggleRow } from "./ProviderToggleRow"
import { SettingBlock } from "./SettingBlock"

const GLOBAL_SCOPE = ""

interface ProvidersPaneProps {
  projectId?: string
  /** The provider whose own screen is open, "" for the list. Held by the screen
   * rather than here so the nav can show where you are. */
  openProvider: string
  onOpenProvider: (id: string) => void
}

// ProvidersPane is the single Providers section: which provider new sessions
// spawn, which ones lich offers at all, and (one level in) everything about
// one of them. It replaced four nav entries (global providers, this project's,
// and a section per enabled provider), so the sidebar no longer grows by one
// line for every agent installed.
//
// The list never leaves the enabled providers on screen alongside the ones that
// are off: turning a provider on is a rare errand, and the rows for seven
// agents nobody uses were most of what the pane showed.
export function ProvidersPane({ projectId, openProvider, onOpenProvider }: ProvidersPaneProps) {
  const providers = useProviders()

  if (providers.length === 0) {
    return <p className="py-5 text-sm text-muted-foreground">Detecting providers…</p>
  }

  // A provider turned off while its screen was open resolves back to the list,
  // the same way the nav resolves a section id it can no longer place.
  const open = enabledProviders(providers).find((provider) => provider.id === openProvider)
  if (open) {
    return (
      <ProviderDetail provider={open} projectId={projectId} onBack={() => onOpenProvider("")} />
    )
  }
  return (
    <ProvidersList providers={providers} projectId={projectId} onOpenProvider={onOpenProvider} />
  )
}

function ProvidersList({
  providers,
  projectId,
  onOpenProvider,
}: {
  providers: ProviderState[]
  projectId?: string
  onOpenProvider: (id: string) => void
}) {
  const enabled = enabledProviders(providers)
  const available = providers.filter((provider) => !provider.enabled)
  // Opened by hand, except on a machine with nothing enabled: there the list of
  // what could be turned on is the only thing this pane has to say.
  const [adding, setAdding] = useState(enabled.length === 0)

  return (
    <>
      <h1 className="mb-4 text-2xl font-semibold text-foreground">Providers</h1>
      <div className="divide-y divide-border">
        <DefaultProviders providers={enabled} projectId={projectId} />
        <div className="py-4">
          <div className="flex flex-col">
            {enabled.map((provider) => (
              <ProviderRow
                key={provider.id}
                provider={provider}
                projectId={projectId}
                onOpen={() => onOpenProvider(provider.id)}
              />
            ))}
          </div>

          {available.length > 0 && (
            <>
              <button
                type="button"
                onClick={() => setAdding(!adding)}
                aria-expanded={adding}
                className="flex w-full items-center gap-2 rounded-md px-2 py-2.5 text-left text-sm text-muted-foreground transition-colors hover:bg-accent/50 hover:text-foreground"
              >
                {adding ? <ChevronDown className="size-4" /> : <Plus className="size-4" />}
                Add provider
                <span className="text-xs">{available.length} more detected</span>
              </button>
              {adding && (
                <div className="pl-2">
                  <div className="flex items-center justify-end pb-1 pr-9">
                    <ProviderRefreshButton />
                  </div>
                  <div className="flex flex-col divide-y divide-border pr-9">
                    {available.map((provider) => (
                      <ProviderToggleRow key={provider.id} provider={provider} />
                    ))}
                  </div>
                </div>
              )}
            </>
          )}
        </div>
      </div>
    </>
  )
}

// DefaultProviders is the two scopes as two labelled rows rather than one
// sentence with two holes in it: the label says which scope a select answers
// for, and the project row says what it would fall back to.
function DefaultProviders({
  providers,
  projectId,
}: {
  providers: ProviderState[]
  projectId?: string
}) {
  const { projects } = useProjects()
  const globalDefault = useDefaultProvider()
  const projectDefault = useStoredProjectDefaultProvider(projectId ?? "")
  const project = projects.find((candidate) => candidate.id === projectId)

  if (providers.length === 0) {
    return null
  }
  const globalName =
    providers.find((provider) => provider.id === globalDefault)?.name ?? globalDefault
  const override = providers.find((provider) => provider.id === projectDefault)

  return (
    <SettingBlock
      title="Default provider"
      description="Which provider implicit session actions spawn: the new-session shortcut, an empty project's New session button, and newly created worktrees."
    >
      <div className="flex flex-col gap-2">
        <div className="flex flex-wrap items-center gap-3">
          <span className="w-28 shrink-0 text-xs text-muted-foreground">All projects</span>
          <ProviderSelect
            providers={providers}
            value={globalDefault}
            ariaLabel="Default provider for all projects"
            onChange={setProviderDefault}
          />
        </div>
        {/* No project open is no second scope to configure, so the row is absent
            rather than present and unanswerable. */}
        {project && projectId && (
          <div className="flex flex-wrap items-center gap-3">
            <span className="w-28 shrink-0 truncate text-xs text-muted-foreground">
              {project.name}
            </span>
            <ProviderSelect
              providers={providers}
              value={override?.id ?? globalDefault}
              ariaLabel={`Default provider for ${project.name}`}
              onChange={(value) => setProjectProviderDefault(projectId, value)}
            />
            {override ? (
              <>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => setProjectProviderDefault(projectId, "")}
                >
                  <X data-icon="inline-start" />
                  Clear
                </Button>
                <span className="text-xs text-muted-foreground">
                  Clearing falls back to {globalName}
                </span>
              </>
            ) : (
              <span className="text-xs text-muted-foreground">
                Following all projects: {globalName}
              </span>
            )}
          </div>
        )}
      </div>
    </SettingBlock>
  )
}

// ProviderRow is one enabled provider on the way in. The whole row navigates,
// which is why the chevron sits at the far right, pointing out of the list
// rather than down into it, and why the switch is layered over the navigating
// button instead of nested inside it: a click that both disables a provider and
// walks into its screen is one click doing two things.
function ProviderRow({
  provider,
  projectId,
  onOpen,
}: {
  provider: ProviderState
  projectId?: string
  onOpen: () => void
}) {
  const plan = usePlanQuotaFor(provider.id)
  const [skipHere] = useStoredFlag(skipPermissionsKey(provider.id, false), GLOBAL_SCOPE)
  const [skipInWorktrees] = useStoredFlag(skipPermissionsKey(provider.id, true), GLOBAL_SCOPE)
  const projectDefault = useStoredProjectDefaultProvider(projectId ?? "")
  const level = skipLevel(skipHere, skipInWorktrees)
  // The plan is what a row is usually opened to check; without one, the binary
  // is the fact that distinguishes this row from the next.
  const summary = planSummary(plan) || provider.binary

  return (
    <div className="group relative flex items-center gap-3 rounded-md px-2 transition-colors hover:bg-accent/50 focus-within:bg-accent/50">
      <button
        type="button"
        onClick={onOpen}
        className="absolute inset-0 rounded-md outline-none"
        aria-label={`Open ${provider.name} settings`}
      />
      <span className="pointer-events-none flex min-w-0 flex-1 items-center gap-3 py-2.5">
        <ProviderIcon kind={provider.id} />
        <span className="shrink-0 text-sm font-medium text-foreground">{provider.name}</span>
        <span
          className={cn(
            "truncate text-xs text-muted-foreground",
            !planSummary(plan) && "font-mono",
          )}
        >
          {summary}
        </span>
      </span>
      <span className="pointer-events-none flex shrink-0 items-center gap-2 text-xs text-muted-foreground">
        {projectDefault === provider.id && <span className="whitespace-nowrap">default here</span>}
        {level !== "never" && (
          <span className="whitespace-nowrap">
            {level === "worktrees" ? "worktrees" : "no prompts"}
          </span>
        )}
      </span>
      <Switch
        checked={provider.enabled}
        onCheckedChange={(checked) => setProviderEnabled(provider.id, checked)}
        aria-label={`Enable ${provider.name}`}
        className="relative shrink-0"
      />
      <ChevronRight className="pointer-events-none size-4 shrink-0 text-muted-foreground" />
    </div>
  )
}
