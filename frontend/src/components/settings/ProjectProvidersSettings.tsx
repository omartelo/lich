import { Button } from "@/components/ui/button"
import {
  enabledProviders,
  setProjectProviderDefault,
  useDefaultProvider,
  useProviders,
  useStoredProjectDefaultProvider,
} from "@/lib/providers-store"
import { useProjects } from "@/providers/projects"
import { ProviderSelect } from "./ProviderSelect"
import { SettingBlock } from "./SettingBlock"

interface ProjectProvidersSettingsProps {
  projectId?: string
}

export function ProjectProvidersSettings({ projectId }: ProjectProvidersSettingsProps) {
  const { projects } = useProjects()
  const providers = useProviders()
  const globalDefault = useDefaultProvider()
  const projectDefault = useStoredProjectDefaultProvider(projectId ?? "")
  const project = projects.find((candidate) => candidate.id === projectId)
  const enabled = enabledProviders(providers)
  const globalName =
    providers.find((provider) => provider.id === globalDefault)?.name ?? globalDefault
  const override = enabled.find((provider) => provider.id === projectDefault)
  const selected = override?.id ?? globalDefault
  const inherited = projectDefault === ""

  if (!project || !projectId) {
    return (
      <p className="py-4 text-sm text-muted-foreground">
        Open a project to configure its providers.
      </p>
    )
  }

  // Detection has not answered yet: the select would offer nothing to pick and
  // the readout would print a raw provider id, so say what is happening instead
  // — the same answer the global pane gives for this state.
  if (enabled.length === 0) {
    return <p className="py-4 text-sm text-muted-foreground">Detecting providers…</p>
  }

  return (
    <SettingBlock
      title={`Default provider for ${project.name}`}
      description="Which provider implicit session actions use in this project: the shortcut, an empty project's New session button, and newly created worktrees."
    >
      <p className="mb-2 text-xs text-muted-foreground">{project.path}</p>
      <div className="flex flex-wrap items-center gap-2">
        <ProviderSelect
          providers={enabled}
          value={selected}
          ariaLabel={`Default provider for ${project.name}`}
          onChange={(value) => setProjectProviderDefault(projectId, value)}
        />
        <Button
          variant="outline"
          disabled={inherited}
          onClick={() => setProjectProviderDefault(projectId, "")}
        >
          Use default
        </Button>
      </div>
      <p className="mt-2 text-xs text-muted-foreground">
        {override ? "Global default" : "Using global default"}: {globalName}
      </p>
    </SettingBlock>
  )
}
