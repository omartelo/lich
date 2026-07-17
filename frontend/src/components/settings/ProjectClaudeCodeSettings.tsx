import { useEffect, useState } from "react"
import { Store } from "@/lib/rpc"
import { useProjects } from "@/lib/projects"
import { Input } from "@/components/ui/input"
import { SettingBlock } from "./SettingBlock"

// Same key as the global section; the scope (project id) is what differs. An
// empty override inherits the global value — the backend resolves project →
// global → $PATH in store.ClaudeBin.
const GLOBAL_SCOPE = ""
const CLAUDE_BIN_KEY = "claude.bin"

// Per-project override for the current project (the one whose Settings screen is
// open). Empty inherits the global custom path.
export function ProjectClaudeCodeSettings({ projectId }: { projectId?: string }) {
  const { projects } = useProjects()
  const project = projects.find((p) => p.id === projectId)
  const [globalBin, setGlobalBin] = useState("")
  const [bin, setBin] = useState("")

  useEffect(() => {
    void Store.GetSetting(CLAUDE_BIN_KEY, GLOBAL_SCOPE).then(setGlobalBin)
  }, [])

  useEffect(() => {
    if (!projectId) {
      return
    }
    void Store.GetSetting(CLAUDE_BIN_KEY, projectId).then(setBin)
  }, [projectId])

  const persist = (value: string) => {
    setBin(value)
    if (projectId) {
      void Store.SetSetting(CLAUDE_BIN_KEY, projectId, value.trim())
    }
  }

  if (!project) {
    return (
      <p className="py-5 text-sm text-muted-foreground">
        No project selected.
      </p>
    )
  }

  return (
    <SettingBlock
      title="Custom path"
      description="Path to the Claude Code binary or a launcher script spawned in this project's terminals. Leave empty to inherit the global custom path."
    >
      <div className="text-sm font-medium text-foreground">{project.name}</div>
      <p className="mb-2 mt-0.5 text-xs text-muted-foreground">{project.path}</p>
      <Input
        value={bin}
        onChange={(event) => persist(event.target.value)}
        placeholder={globalBin || "claude"}
        spellCheck={false}
        aria-label={`Claude Code path for ${project.name}`}
        className="w-96 max-w-full font-mono"
      />
    </SettingBlock>
  )
}
