import { useEffect, useState } from "react"
import { Store } from "@/lib/rpc"
import { useProjects } from "@/lib/projects"
import { binKey } from "@/lib/providers-store"
import { Input } from "@/components/ui/input"
import { SettingBlock } from "./SettingBlock"

const GLOBAL_SCOPE = ""

// ProviderBinSettings is the config section a provider gets when enabled: the
// custom path to its binary or launcher script, as a global default plus an
// optional per-project override. Empty inherits: project → global → the
// provider's own name on $PATH. Same keys the Go store resolves (see binKey).
export function ProviderBinSettings({
  providerId,
  projectId,
}: {
  providerId: string
  projectId?: string
}) {
  const { projects } = useProjects()
  const project = projects.find((p) => p.id === projectId)
  const key = binKey(providerId)
  const [globalBin, setGlobalBin] = useState("")
  const [projectBin, setProjectBin] = useState("")

  useEffect(() => {
    void Store.GetSetting(key, GLOBAL_SCOPE).then(setGlobalBin)
  }, [key])

  useEffect(() => {
    if (!projectId) {
      return
    }
    void Store.GetSetting(key, projectId).then(setProjectBin)
  }, [key, projectId])

  const persistGlobal = (value: string) => {
    setGlobalBin(value)
    void Store.SetSetting(key, GLOBAL_SCOPE, value.trim())
  }

  const persistProject = (value: string) => {
    setProjectBin(value)
    if (projectId) {
      void Store.SetSetting(key, projectId, value.trim())
    }
  }

  return (
    <>
      <SettingBlock
        title="Custom path"
        description="Path to the binary or a launcher script spawned in each terminal. Leave empty to run it from your $PATH."
      >
        <Input
          value={globalBin}
          onChange={(event) => persistGlobal(event.target.value)}
          placeholder={providerId}
          spellCheck={false}
          aria-label={`${providerId} custom path`}
          className="w-96 max-w-full font-mono"
        />
      </SettingBlock>

      {project && (
        <SettingBlock
          title={`Override for ${project.name}`}
          description="A path used only in this project's terminals. Leave empty to inherit the global custom path."
        >
          <p className="mb-2 text-xs text-muted-foreground">{project.path}</p>
          <Input
            value={projectBin}
            onChange={(event) => persistProject(event.target.value)}
            placeholder={globalBin || providerId}
            spellCheck={false}
            aria-label={`${providerId} path for ${project.name}`}
            className="w-96 max-w-full font-mono"
          />
        </SettingBlock>
      )}
    </>
  )
}
