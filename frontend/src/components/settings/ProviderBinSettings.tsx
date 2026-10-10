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
import { useT } from "@/lib/i18n/i18n"
import { useStoredFlag } from "@/lib/use-stored-setting"
import { Button } from "@/components/ui/button"
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group"
import { ConfirmDialog } from "@/components/ConfirmDialog"
import { PlanUsageSetting } from "./PlanUsageSetting"
import { ProviderBinary } from "./ProviderBinary"
import { RestoreSetting } from "./RestoreSetting"
import { SettingBlock } from "./SettingBlock"
import { SubagentCardsSetting } from "./SubagentCardsSetting"
import { UltracodeSetting } from "./UltracodeSetting"

const GLOBAL_SCOPE = ""

// How far the provider runs without asking, ordered by risk.
const SKIP_LEVELS: SkipLevel[] = ["never", "worktrees", "everywhere"]

// ProviderBinSettings is the config section a provider gets when enabled: what
// its plan has left, which binary its sessions spawn, how far it runs without
// asking, whether it starts in ultracode, and what its restored cards do on
// first open. Footer visibility is configured globally in Appearance.
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
  const t = useT()
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
  const consequence = t(`settings.providerBinSettings.skip.${level}.consequence`)
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
          title={t("settings.providerBinSettings.skipTitle")}
          description={t("settings.providerBinSettings.skipDescription", {
            provider: providerName,
            flag: skipFlag,
          })}
        >
          <ToggleGroup
            value={[level]}
            // An empty array is the pressed rung being pressed again. There is
            // no fourth answer to fall back to, so it stays where it was.
            onValueChange={(next) => next[0] && chooseLevel(next[0] as SkipLevel)}
            spacing={1}
            aria-label={t("settings.providerBinSettings.skipLabel", { provider: providerName })}
            className="border border-border p-[0.1875rem]"
          >
            {SKIP_LEVELS.map((rung) => (
              <ToggleGroupItem key={rung} value={rung} size="sm">
                {t(`settings.providerBinSettings.skip.${rung}.label`)}
              </ToggleGroupItem>
            ))}
          </ToggleGroup>
          <p className="mt-2 text-xs text-muted-foreground">{consequence}</p>

          <ConfirmDialog
            open={pendingLevel !== null}
            onCancel={() => setPendingLevel(null)}
            title={t("settings.providerBinSettings.confirmTitle", {
              level: pendingLevel
                ? t(`settings.providerBinSettings.skip.${pendingLevel}.label`).toLowerCase()
                : "",
            })}
            description={t("settings.providerBinSettings.confirmDescription", {
              provider: providerName,
              flag: skipFlag,
              consequence: pendingLevel
                ? t(`settings.providerBinSettings.skip.${pendingLevel}.consequence`)
                : "",
            })}
          >
            <Button
              variant="destructive"
              onClick={() => {
                if (pendingLevel) {
                  setLevel(pendingLevel)
                }
                setPendingLevel(null)
              }}
            >
              {t("settings.providerBinSettings.confirm")}
            </Button>
          </ConfirmDialog>
        </SettingBlock>
      )}

      <UltracodeSetting providerId={providerId} providerName={providerName} />
      <SubagentCardsSetting providerId={providerId} providerName={providerName} />

      <RestoreSetting providerId={providerId} providerName={providerName} />
    </>
  )
}
