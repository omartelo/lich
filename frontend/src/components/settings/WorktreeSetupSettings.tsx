import { useEffect, useState } from "react"
import { Store } from "@/lib/rpc"
import { useProjects } from "@/lib/projects"
import { Input } from "@/components/ui/input"
import { SettingBlock } from "./SettingBlock"

// Same key the Go store resolves (internal/store worktreeSetupKey).
export const WORKTREE_SETUP_KEY = "worktree.setup"

// WorktreeSetupSettings edits the project's worktree setup script: a shell
// command run in a new worktree session's terminal ahead of the provider,
// right after the worktree is created — dependency installs and generated
// files, the things a fresh checkout is missing. Project-scoped only, no
// global value: a setup command is repo-specific.
export function WorktreeSetupSettings({ projectId }: { projectId?: string }) {
  const { projects } = useProjects()
  const project = projects.find((p) => p.id === projectId)
  const [script, setScript] = useState("")

  useEffect(() => {
    if (!projectId) {
      return
    }
    void Store.GetSetting(WORKTREE_SETUP_KEY, projectId).then(setScript)
  }, [projectId])

  const persist = (value: string) => {
    setScript(value)
    if (projectId) {
      void Store.SetSetting(WORKTREE_SETUP_KEY, projectId, value.trim())
    }
  }

  if (!project) {
    return (
      <p className="py-4 text-sm text-muted-foreground">
        Open a project to configure its worktree setup.
      </p>
    )
  }

  return (
    <SettingBlock
      title={`Setup script for ${project.name}`}
      description={
        "Runs in every new worktree's terminal before the agent starts — chain steps with &&. " +
        "The session opens even if it fails. Gitignored .env* files are copied into the new " +
        "worktree automatically; a .worktreeinclude file at the repo root overrides the patterns."
      }
    >
      <p className="mb-2 text-xs text-muted-foreground">{project.path}</p>
      <Input
        value={script}
        onChange={(event) => persist(event.target.value)}
        placeholder="pnpm install"
        spellCheck={false}
        aria-label={`Worktree setup script for ${project.name}`}
        className="w-96 max-w-full font-mono"
      />
    </SettingBlock>
  )
}
