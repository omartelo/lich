import { useState } from "react"
import {
  climbsToRiskier,
  skipLevel,
  skipLevelPair,
  skipPermissionFlags,
  skipPermissionsKey,
  SKIP_RISK_ORDER,
  type SkipLevel,
} from "@/lib/providers-store"
import { useStoredFlag } from "@/lib/use-stored-setting"
import { Button } from "@/components/ui/button"
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group"
import { ConfirmDialog } from "@/components/ConfirmDialog"
import { PlanUsageSetting } from "./PlanUsageSetting"
import { ProviderBinary } from "./ProviderBinary"
import { SettingBlock } from "./SettingBlock"

const GLOBAL_SCOPE = ""

// How far the provider runs without asking, ordered by risk.
const SKIP_LEVELS: { level: SkipLevel; label: string; consequence: string }[] = [
  {
    level: "never",
    label: "Never",
    consequence: "Every edit and command waits for you, in every checkout.",
  },
  {
    level: "worktrees",
    label: "Worktrees only",
    consequence: "Sessions in the project directory keep asking.",
  },
  {
    level: "everywhere",
    label: "Everywhere",
    consequence: "Including the tree you work in. Nothing will ask.",
  },
]

// ProviderBinSettings is the config section a provider gets when enabled: what
// its plan has left, which binary its sessions spawn, and how far it runs
// without asking. Footer visibility is configured globally in Appearance.
export function ProviderBinSettings({
  providerId,
  providerName,
  providerBin,
  projectId,
}: {
  providerId: string
  providerName: string
  providerBin: string
  projectId?: string
}) {
  // Off until the store answers, and that direction is not a detail: this is
  // the switch that hands an agent the machine, so the unknown state can never
  // be drawn as the permissive one.
  const [skipHere, setSkipHere] = useStoredFlag(skipPermissionsKey(providerId, false), GLOBAL_SCOPE)
  const [skipInWorktrees, setSkipInWorktrees] = useStoredFlag(
    skipPermissionsKey(providerId, true),
    GLOBAL_SCOPE,
  )
  // The rung a click asked for and the write is waiting on. Null while nothing
  // is pending, which is every click that does not climb.
  const [pendingLevel, setPendingLevel] = useState<SkipLevel | null>(null)
  const skipFlag = skipPermissionFlags[providerId]
  const level = skipLevel(skipHere, skipInWorktrees)
  const consequence = SKIP_LEVELS.find((rung) => rung.level === level)?.consequence ?? ""
  const pendingRung = SKIP_LEVELS.find((rung) => rung.level === pendingLevel)
  // One rung writes both keys, always: the pair is the storage, the rung is the
  // choice. Writing only the one that changed would leave the other holding an
  // answer to a question the user is no longer being asked.
  const setLevel = (next: SkipLevel) => {
    const pair = skipLevelPair(next)
    setSkipHere(pair.here)
    setSkipInWorktrees(pair.worktrees)
  }

  // Climbing hands the agent more of the machine and is confirmed once; coming
  // back down is written straight through, because taking the automation away
  // must never be the harder direction.
  const chooseLevel = (next: SkipLevel) => {
    if (climbsToRiskier(SKIP_RISK_ORDER, level, next)) {
      setPendingLevel(next)
      return
    }
    setLevel(next)
  }

  return (
    <>
      {/* What the plan has left comes first: it is the state of this provider,
          and everything under it is configuration. Renders itself away for a
          provider that meters no subscription. */}
      <PlanUsageSetting providerId={providerId} />

      <ProviderBinary
        providerId={providerId}
        providerName={providerName}
        providerBin={providerBin}
        projectId={projectId}
      />

      {/* Never unless the user says otherwise. A worktree is its own rung
          because it is a checkout you can throw away, while the project
          directory is the one you work in. Absent for a provider whose flag
          lich has no spelling for — the control would store a setting nothing
          reads. */}
      {skipFlag && (
        <SettingBlock
          title="Skip permission prompts"
          description={`How far ${providerName} runs without asking: it edits files, runs commands and installs things unconfirmed, and lich spawns it with ${skipFlag}.`}
        >
          <ToggleGroup
            value={[level]}
            // An empty array is the pressed rung being pressed again. There is
            // no fourth answer to fall back to, so it stays where it was.
            onValueChange={(next) => next[0] && chooseLevel(next[0] as SkipLevel)}
            spacing={1}
            aria-label={`How far ${providerName} runs without asking`}
            className="border border-border p-[0.1875rem]"
          >
            {SKIP_LEVELS.map((rung) => (
              <ToggleGroupItem key={rung.level} value={rung.level} size="sm">
                {rung.label}
              </ToggleGroupItem>
            ))}
          </ToggleGroup>
          <p className="mt-2 text-xs text-muted-foreground">{consequence}</p>

          <ConfirmDialog
            open={pendingRung !== undefined}
            onCancel={() => setPendingLevel(null)}
            title={`Skip permission prompts: ${pendingRung?.label.toLowerCase()}`}
            description={`${providerName} will edit files, run commands and install things unconfirmed, spawned with ${skipFlag}. ${pendingRung?.consequence}`}
          >
            <Button
              variant="destructive"
              onClick={() => {
                if (pendingRung) {
                  setLevel(pendingRung.level)
                }
                setPendingLevel(null)
              }}
            >
              Skip prompts
            </Button>
          </ConfirmDialog>
        </SettingBlock>
      )}
    </>
  )
}
