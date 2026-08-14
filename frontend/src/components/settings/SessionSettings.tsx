import { ProviderIcon } from "@/components/ProviderIcon"
import { Button } from "@/components/ui/button"
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import {
  enabledProviders,
  setProjectProviderDefault,
  useDefaultProvider,
  useProviders,
  useStoredProjectDefaultProvider,
} from "@/lib/providers-store"
import type { ProviderKind } from "@/lib/session/sessions"
import { useProjects } from "@/providers/projects"
import { SettingBlock } from "./SettingBlock"

// SessionSettings holds the project half of provider selection. The global
// fallback stays in Providers: an empty value here is meaningful inheritance,
// not an unconfigured error, which is why clearing gets its own explicit action.
export function SessionSettings({ projectId }: { projectId?: string }) {
  const { projects } = useProjects()
  const providers = useProviders()
  const globalDefault = useDefaultProvider()
  const projectDefault = useStoredProjectDefaultProvider(projectId ?? "")
  const project = projects.find((candidate) => candidate.id === projectId)
  const enabled = enabledProviders(providers)
  const globalName =
    providers.find((provider) => provider.id === globalDefault)?.name ?? globalDefault
  const selected = enabled.some((provider) => provider.id === projectDefault) ? projectDefault : ""
  const items = Object.fromEntries(enabled.map((provider) => [provider.id, provider.name]))

  if (!project || !projectId) {
    return (
      <p className="py-4 text-sm text-muted-foreground">
        Open a project to configure its sessions.
      </p>
    )
  }

  return (
    <SettingBlock
      title={`Default harness for ${project.name}`}
      description="Which harness implicit session actions use in this project: the shortcut, an empty project's New session button, and newly created worktrees."
    >
      <p className="mb-2 text-xs text-muted-foreground">{project.path}</p>
      <div className="flex flex-wrap items-center gap-2">
        <Select
          value={selected}
          items={items}
          onValueChange={(value) =>
            value && setProjectProviderDefault(projectId, value as ProviderKind)
          }
        >
          <SelectTrigger className="w-64">
            <SelectValue placeholder="Select a harness" />
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
        <Button
          variant="outline"
          disabled={!projectDefault}
          onClick={() => setProjectProviderDefault(projectId, "")}
        >
          Use default
        </Button>
      </div>
      <p className="mt-2 text-xs text-muted-foreground">
        {selected ? "Global default" : "Using global default"}: {globalName}
      </p>
    </SettingBlock>
  )
}
