import { useState } from "react"
import {
  climbsToRiskier,
  footerReadout,
  footerReadoutPair,
  skipLevel,
  skipLevelPair,
  skipPermissionFlags,
  skipPermissionsKey,
  SKIP_RISK_ORDER,
  type FooterReadout,
  type SkipLevel,
} from "@/lib/providers-store"
import { useSettings } from "@/providers/settings"
import { setCostReadout } from "@/lib/cost-readout-store"
import { useCostReadout } from "@/lib/use-cost-readout"
import { useStoredFlag } from "@/lib/use-stored-setting"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group"
import { ConfirmDialog } from "@/components/ConfirmDialog"
import { PlanUsageSetting } from "./PlanUsageSetting"
import { ProviderBinary } from "./ProviderBinary"
import { SettingBlock } from "./SettingBlock"

const GLOBAL_SCOPE = ""

// Each rung says what lands in the footer, and the line under it says what that
// means — the half of the answer the two switches made people assemble in their
// heads.
const FOOTER_READOUTS: { level: FooterReadout; label: string; consequence: string }[] = [
  {
    level: "off",
    label: "Nothing",
    consequence: "The footer keeps only the branch, the path and the clock.",
  },
  {
    level: "context",
    label: "Model & context",
    consequence: "The model a session runs, and a ring showing how full its context window is.",
  },
  {
    level: "cost",
    label: "And cost",
    consequence:
      "Plus what the session has cost at API prices, summed from its transcript. That figure only means something when you are billed per token, not on a subscription.",
  },
]

// The same shape for how far the provider runs without asking, ordered by risk.
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
// without asking. Claude Code and Codex add the footer readout because their
// transcripts report the model and context-window usage.
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
  const { showContextUsage, setShowContextUsage, costBudget, setCostBudget } = useSettings()
  const showCost = useCostReadout()
  // The field keeps the raw string so a half-typed "1." survives the keystroke;
  // the stored budget is the parsed value, and an emptied field is no budget.
  const [budget, setBudget] = useState(() => (costBudget > 0 ? String(costBudget) : ""))
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
  const readout = footerReadout(showContextUsage, showCost)
  const availableReadouts =
    providerId === "codex"
      ? FOOTER_READOUTS.filter((rung) => rung.level !== "cost")
      : FOOTER_READOUTS
  // Cost is a global preference but Codex has no transcript cost reader. If
  // Claude enabled it, the Codex pane still truthfully shows its highest rung.
  const visibleReadout = providerId === "codex" && readout === "cost" ? "context" : readout
  const readoutConsequence =
    availableReadouts.find((rung) => rung.level === visibleReadout)?.consequence ?? ""

  // One rung writes both settings, for the same reason the permission ladder
  // does: the pair is the storage, the rung is the choice.
  const setReadout = (next: FooterReadout) => {
    const pair = footerReadoutPair(next)
    setShowContextUsage(pair.context)
    setCostReadout(pair.cost)
  }

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

  const persistBudget = (value: string) => {
    setBudget(value)
    setCostBudget(Number(value))
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

      {/* Only providers with a context-window transcript reader carry this
          control; the underlying preference stays global. */}
      {(providerId === "claude" || providerId === "codex") && (
        <>
          <SettingBlock
            title="Session readout in the footer"
            description="How much of what a session is spending the footer carries, read from its transcript."
          >
            <ToggleGroup
              value={[visibleReadout]}
              onValueChange={(next) => next[0] && setReadout(next[0] as FooterReadout)}
              spacing={1}
              aria-label="What the footer says about the session"
              className="border border-border p-[3px]"
            >
              {availableReadouts.map((rung) => (
                <ToggleGroupItem key={rung.level} value={rung.level} size="sm">
                  {rung.label}
                </ToggleGroupItem>
              ))}
            </ToggleGroup>
            <p className="mt-2 max-w-prose text-xs text-muted-foreground">{readoutConsequence}</p>
          </SettingBlock>

          {/* A ceiling is only ever seen through the readout, so it is offered
              beside it and hidden with it. */}
          {providerId === "claude" && readout === "cost" && (
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
        </>
      )}

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
            className="border border-border p-[3px]"
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
