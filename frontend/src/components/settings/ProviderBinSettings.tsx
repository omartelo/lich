import { useEffect, useState } from "react"
import { Store, System } from "@/lib/rpc"
import {
  footerReadout,
  footerReadoutPair,
  skipLevel,
  skipLevelPair,
  sandboxKey,
  sandboxLevel,
  skipPermissionFlags,
  skipPermissionsKey,
  type FooterReadout,
  type SandboxLevel,
  type SkipLevel,
} from "@/lib/providers-store"
import { useSettings } from "@/providers/settings"
import { setCostReadout } from "@/lib/cost-readout-store"
import { useCostReadout } from "@/lib/use-cost-readout"
import { Input } from "@/components/ui/input"
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group"
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

// The same shape again for which sessions run confined, ordered by how much of
// the machine a session can reach. "Ask each time" sits second because a session
// nobody answered for runs unconfined, exactly like "Off".
const SANDBOX_LEVELS: { level: SandboxLevel; label: string; consequence: string }[] = [
  {
    level: "off",
    label: "Off",
    consequence: "Sessions run straight on the machine, reaching everything you can reach.",
  },
  {
    level: "ask",
    label: "Ask each time",
    consequence:
      "Every new session asks before it opens. Nothing is remembered — a session opened any other way, by a delegation or an MCP tool, is not confined.",
  },
  {
    level: "worktrees",
    label: "Worktrees only",
    consequence: "Sessions in the project directory run on the machine.",
  },
  {
    level: "everywhere",
    label: "Everywhere",
    consequence: "Including the tree you work in. Every session opens confined.",
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

// useSandboxLevel is the stored sandbox rung for one provider in one scope, read
// once and written through on every change. It reads "off" until the value
// arrives, which is also what an unreadable value means: the unknown state is
// drawn as the one lich has always behaved like, never as one that silently
// starts confining sessions.
function useSandboxLevel(providerId: string, projectId: string) {
  const key = sandboxKey(providerId)
  const [level, setLevel] = useState<SandboxLevel>("off")

  useEffect(() => {
    void Store.GetSetting(key, projectId).then((value) => setLevel(sandboxLevel(value)))
  }, [key, projectId])

  const choose = (next: SandboxLevel) => {
    setLevel(next)
    void Store.SetSetting(key, projectId, next)
  }
  return [level, choose] as const
}

// useSandboxAvailable answers whether this machine can confine anything at all —
// bubblewrap on Linux, sandbox-exec on macOS. Undefined until it arrives, so the
// control is absent rather than briefly drawn as unavailable on every mount.
function useSandboxAvailable() {
  const [available, setAvailable] = useState<boolean | undefined>(undefined)
  useEffect(() => {
    void System.SandboxAvailable().then(setAvailable)
  }, [])
  return available
}

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

// ProviderBinSettings is the config section a provider gets when enabled: what
// its plan has left, which binary its sessions spawn, and how far it runs
// without asking. Claude Code adds the footer readout, which is the only
// provider reporting one.
export function ProviderBinSettings({
  providerId,
  providerName,
  projectId,
}: {
  providerId: string
  providerName: string
  projectId?: string
}) {
  const { showContextUsage, setShowContextUsage, costBudget, setCostBudget } = useSettings()
  const showCost = useCostReadout()
  // The field keeps the raw string so a half-typed "1." survives the keystroke;
  // the stored budget is the parsed value, and an emptied field is no budget.
  const [budget, setBudget] = useState(() => (costBudget > 0 ? String(costBudget) : ""))
  const [skipHere, setSkipHere] = useSkipPermissions(providerId, false)
  const [skipInWorktrees, setSkipInWorktrees] = useSkipPermissions(providerId, true)
  // The sandbox is scoped to whichever pane this is: the project's own rung when
  // opened from a project, the global one from the hub.
  const [sandbox, setSandbox] = useSandboxLevel(providerId, projectId ?? GLOBAL_SCOPE)
  const sandboxAvailable = useSandboxAvailable()
  const sandboxConsequence =
    SANDBOX_LEVELS.find((rung) => rung.level === sandbox)?.consequence ?? ""
  const skipFlag = skipPermissionFlags[providerId]
  const level = skipLevel(skipHere, skipInWorktrees)
  const consequence = SKIP_LEVELS.find((rung) => rung.level === level)?.consequence ?? ""
  const readout = footerReadout(showContextUsage, showCost)
  const readoutConsequence =
    FOOTER_READOUTS.find((rung) => rung.level === readout)?.consequence ?? ""

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

      <ProviderBinary providerId={providerId} providerName={providerName} projectId={projectId} />

      {/* Only Claude Code reports context-window usage (via the lich plugin), so
          the footer readout control lives in its section, not the generic hub. */}
      {providerId === "claude" && (
        <>
          <SettingBlock
            title="Session readout in the footer"
            description="How much of what a session is spending the footer carries, read from its transcript."
          >
            <ToggleGroup
              value={[readout]}
              onValueChange={(next) => next[0] && setReadout(next[0] as FooterReadout)}
              spacing={1}
              aria-label="What the footer says about the session"
              className="border border-border p-[3px]"
            >
              {FOOTER_READOUTS.map((rung) => (
                <ToggleGroupItem key={rung.level} value={rung.level} size="sm">
                  {rung.label}
                </ToggleGroupItem>
              ))}
            </ToggleGroup>
            <p className="mt-2 max-w-prose text-xs text-muted-foreground">{readoutConsequence}</p>
          </SettingBlock>

          {/* A ceiling is only ever seen through the readout, so it is offered
              beside it and hidden with it. */}
          {readout === "cost" && (
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

      {/* Directly above the permission ladder: the two answer the same worry
          from opposite ends, and are read together. Absent on a machine with no
          backend for it — a control that saves a setting nothing can act on is
          worse than no control. */}
      {sandboxAvailable && (
        <SettingBlock
          title="Sandbox"
          description={`Run ${providerName} confined: an empty home holding only its own state, the machine read-only, and writes only inside the checkout. The network stays on — the agent still reaches its API, and lich still hears its hooks.`}
        >
          <ToggleGroup
            value={[sandbox]}
            onValueChange={(next) => next[0] && setSandbox(next[0] as SandboxLevel)}
            spacing={1}
            aria-label={`Which ${providerName} sessions run confined`}
            className="border border-border p-[3px]"
          >
            {SANDBOX_LEVELS.map((rung) => (
              <ToggleGroupItem key={rung.level} value={rung.level} size="sm">
                {rung.label}
              </ToggleGroupItem>
            ))}
          </ToggleGroup>
          <p className="mt-2 max-w-prose text-xs text-muted-foreground">{sandboxConsequence}</p>
        </SettingBlock>
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
            onValueChange={(next) => next[0] && setLevel(next[0] as SkipLevel)}
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
        </SettingBlock>
      )}
    </>
  )
}
