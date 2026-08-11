import { useEffect, useState } from "react"
import { Store } from "@/lib/rpc"
import { useProjects } from "@/providers/projects"
import { binKey, skipPermissionsKey } from "@/lib/providers-store"
import { useSettings } from "@/providers/settings"
import { setCostReadout } from "@/lib/cost-readout-store"
import { useCostReadout } from "@/lib/use-cost-readout"
import { Input } from "@/components/ui/input"
import { Switch } from "@/components/ui/switch"
import { SettingBlock } from "./SettingBlock"

const GLOBAL_SCOPE = ""

// useSkipPermissions is the stored "run without permission prompts" flag for one
// checkout scope: read once, written through on every toggle. It reads off until
// the value arrives — this is the switch that hands an agent the machine, so the
// unknown state must never be drawn as the permissive one.
function useSkipPermissions(providerId: string, worktree: boolean) {
  const key = skipPermissionsKey(providerId, worktree)
  const [on, setOn] = useState(false)

  useEffect(() => {
    void Store.GetSetting(key, GLOBAL_SCOPE).then((value) => setOn(value === "true"))
  }, [key])

  const toggle = (next: boolean) => {
    setOn(next)
    void Store.SetSetting(key, GLOBAL_SCOPE, String(next))
  }
  return [on, toggle] as const
}

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
  const { showContextUsage, setShowContextUsage, costBudget, setCostBudget } = useSettings()
  const showCost = useCostReadout()
  const project = projects.find((p) => p.id === projectId)
  const key = binKey(providerId)
  const [globalBin, setGlobalBin] = useState("")
  const [projectBin, setProjectBin] = useState("")
  // The field keeps the raw string so a half-typed "1." survives the keystroke;
  // the stored budget is the parsed value, and an emptied field is no budget.
  const [budget, setBudget] = useState(() => (costBudget > 0 ? String(costBudget) : ""))
  const [skipHere, setSkipHere] = useSkipPermissions(providerId, false)
  const [skipInWorktrees, setSkipInWorktrees] = useSkipPermissions(providerId, true)

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

  const persistBudget = (value: string) => {
    setBudget(value)
    setCostBudget(Number(value))
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

      {/* Only Claude Code reports context-window usage (via the lich plugin), so
          the footer readout toggles live in its section, not the generic hub. */}
      {providerId === "claude" && (
        <>
          <SettingBlock
            title="Model & context in the footer"
            description="Show this session's model and context-window usage in the footer — the model name plus a ring with the percent, read from the transcript."
          >
            <Switch
              checked={showContextUsage}
              onCheckedChange={setShowContextUsage}
              aria-label="Show model and context usage in the footer"
            />
          </SettingBlock>

          <SettingBlock
            title="Session cost in the footer"
            description="Show what each session has cost at API prices, summed from its transcript. Leave this off on a subscription plan — the figure only means something when you are billed per token."
          >
            <Switch
              checked={showCost}
              onCheckedChange={setCostReadout}
              aria-label="Show session cost in the footer"
            />
          </SettingBlock>

          {/* A ceiling is only ever seen through the readout, so it is offered
              beside it and hidden with it. */}
          {showCost && (
            <SettingBlock
              title="Spend ceiling"
              description="Colour the cost in the footer as a session approaches this many dollars — amber at 80%, red at 95%. It is a warning, not a limit: nothing is stopped, and the figure it watches is API pricing from a table. Leave empty for none."
            >
              <Input
                type="number"
                min={0}
                step={1}
                value={budget}
                onChange={(event) => persistBudget(event.target.value)}
                placeholder="No ceiling"
                aria-label="Session spend ceiling in dollars"
                className="w-40 font-mono"
              />
            </SettingBlock>
          )}

          {/* Both off unless the user says otherwise, and separate on purpose:
              a worktree is a checkout you can throw away, the project directory
              is the one you work in. */}
          <SettingBlock
            title="Skip permission prompts"
            description="Spawn Claude Code with --dangerously-skip-permissions in this project's own checkout. It edits files, runs commands and installs things without asking first, and whatever it gets wrong lands in the tree you work in."
          >
            <Switch
              checked={skipHere}
              onCheckedChange={setSkipHere}
              aria-label="Skip permission prompts in the project directory"
            />
          </SettingBlock>

          <SettingBlock
            title="Skip permission prompts in worktrees"
            description="The same flag for sessions started in a git worktree. That checkout is separate from your working tree, so a mistake stays in the branch until you merge it."
          >
            <Switch
              checked={skipInWorktrees}
              onCheckedChange={setSkipInWorktrees}
              aria-label="Skip permission prompts in worktree sessions"
            />
          </SettingBlock>
        </>
      )}
    </>
  )
}
