import { useState } from "react"
import { LoaderCircle, Puzzle } from "lucide-react"
import { Button } from "@/components/ui/button"
import { ProviderIcon } from "@/components/ProviderIcon"
import { SettingBlock } from "./SettingBlock"
import { AgentPlugin } from "@/lib/rpc"
import { CODEX_TRUST_HINT, CRUSH_SCOPE_HINT, RESTART_HINT } from "@/lib/update/plugin-gate"
import { runWithToast } from "@/lib/toast-async"
import type { PluginStatus } from "@/lib/api-types"

interface PluginSettingProps {
  /** One entry per provider that can run the plugin; null until Status answers. */
  statuses: PluginStatus[] | null
  /** Re-read Status after a mutation, so the rows show what just changed. */
  onRefresh: () => Promise<void>
}

// PluginSetting is the plugin's row per provider CLI: its installed version, the
// action that closes the gap, and — for a CLI the machine does not have — the
// plain reason there is nothing to do.
export function PluginSetting({ statuses, onRefresh }: PluginSettingProps) {
  const [busy, setBusy] = useState(false)
  const [result, setResult] = useState("")

  const run = async (call: () => Promise<null>, progress: string, done: string, failed: string) => {
    setBusy(true)
    if (await runWithToast(progress, call, done, failed)) {
      await onRefresh()
    }
    setBusy(false)
  }

  const check = async () => {
    setBusy(true)
    setResult("")
    try {
      await onRefresh()
      setResult("Checked.")
    } catch {
      setResult("Check failed — are you online?")
    } finally {
      setBusy(false)
    }
  }

  const spinner = <LoaderCircle className="size-4 animate-spin" />
  const showTrustHint = statuses?.some((s) => s.provider === "codex" && s.installed)
  const showCrushHint = statuses?.some((s) => s.provider === "crush" && s.installed)

  return (
    <SettingBlock
      icon={<Puzzle className="size-4" />}
      title="lich plugin"
      description="Session status, titles, git refresh and resume, inside the provider CLIs that can run it."
    >
      <div className="flex w-full flex-col gap-1">
        {statuses?.map((status) => (
          <div key={status.provider} className="flex items-center gap-2.5 py-1 text-sm">
            <ProviderIcon kind={status.provider} size={15} />
            <span>{status.name}</span>
            <span className="ml-auto text-xs text-muted-foreground">
              {!status.available
                ? "CLI not installed"
                : status.installed
                  ? `v${status.installedVersion}`
                  : "Plugin not installed"}
            </span>
            {status.available && !status.installed && (
              <Button
                size="sm"
                onClick={() =>
                  void run(
                    () => AgentPlugin.Install(status.provider),
                    `Installing lich plugin for ${status.name}…`,
                    `Plugin installed — ${RESTART_HINT}`,
                    "Install failed",
                  )
                }
                disabled={busy}
              >
                {busy ? spinner : null}
                Install
              </Button>
            )}
            {status.installed && status.updateAvailable && (
              <Button
                size="sm"
                variant="outline"
                onClick={() =>
                  void run(
                    () => AgentPlugin.Update(status.provider),
                    `Updating lich plugin for ${status.name}…`,
                    `Plugin updated — ${RESTART_HINT}`,
                    "Update failed",
                  )
                }
                disabled={busy}
              >
                {busy ? spinner : null}
                Update to v{status.latestVersion}
              </Button>
            )}
          </div>
        ))}
        {showTrustHint && (
          <p className="text-xs text-muted-foreground">Codex: {CODEX_TRUST_HINT}</p>
        )}
        {showCrushHint && (
          <p className="text-xs text-muted-foreground">Crush: {CRUSH_SCOPE_HINT}</p>
        )}
        <div className="flex items-center gap-3 pt-1">
          <Button size="sm" variant="outline" onClick={() => void check()} disabled={busy}>
            Check for updates
          </Button>
          {result && <span className="text-xs text-muted-foreground">{result}</span>}
        </div>
      </div>
    </SettingBlock>
  )
}
