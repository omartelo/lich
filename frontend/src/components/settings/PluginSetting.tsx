import { useState } from "react"
import { LoaderCircle, Puzzle } from "lucide-react"
import { Button } from "@/components/ui/button"
import { ProviderIcon } from "@/components/ProviderIcon"
import { SettingBlock } from "./SettingBlock"
import { AgentPlugin } from "@/lib/rpc"
import {
  CODEX_TRUST_HINT,
  CRUSH_SCOPE_HINT,
  CURSOR_SHARED_PLUGIN_HINT,
  NO_APPROVAL_EVENT_HINT,
  RESTART_HINT,
} from "@/lib/update/plugin-gate"
import { useT } from "@/lib/i18n/i18n"
import { runWithToast } from "@/lib/toast-async"
import { useRemoteResource } from "@/lib/use-remote-resource"
import type { PluginStatus } from "@/lib/api-types"

// PluginSetting is the plugin's row per provider CLI: its installed version, the
// action that closes the gap, and — for a CLI the machine does not have — the
// plain reason there is nothing to do.
export function PluginSetting() {
  const t = useT()
  const [busy, setBusy] = useState(false)
  // Whether the user has asked for a check since this pane was mounted. The
  // outcome itself is not stored: it is whatever the refresh left in `error`,
  // read at render, so a later failure cannot sit under an older "Checked.".
  const [checked, setChecked] = useState(false)
  // One entry per provider that can run the plugin; null until Status answers.
  // That read costs ~180 ms and every row here is drawn from it, so it is filed
  // like the reads above it: coming back to Updates paints the last answer and
  // revalidates underneath. The call takes no arguments, so the caller's own
  // name is the whole of what identifies the answer it files.
  const {
    data: statuses,
    error,
    refresh,
  } = useRemoteResource<PluginStatus[] | null>("agent-plugin-status", () => AgentPlugin.Status(), {
    empty: null,
    cache: "settings.pluginStatus",
  })

  const run = async (call: () => Promise<null>, progress: string, done: string, failed: string) => {
    if (await runWithToast(progress, call, done, failed)) {
      // The filed answer is replaced by this read, so the next visit shows what
      // the action just changed rather than the rows it changed away from.
      await refresh()
    }
  }

  // What the button under the pointer is really for. The row is drawn from a
  // filed answer, so a plugin installed or removed from a terminal while the
  // user was elsewhere offers the wrong thing until this visit's read lands:
  // the click re-asks first and acts on the row that comes back, so an Update
  // on a plugin since removed installs it instead, and an Install of one that
  // is already there only repaints. Verify before acting, never before showing
  // — the rows go on painting from the cache, and only the click pays a read.
  const act = async (provider: string, name: string) => {
    setBusy(true)
    const fresh = (await refresh())?.find((row) => row.provider === provider)
    if (fresh?.available && !fresh.installed) {
      await run(
        () => AgentPlugin.Install(provider),
        t("settings.pluginSetting.installing", { name }),
        t("settings.pluginSetting.installed", { hint: RESTART_HINT }),
        t("settings.pluginSetting.installFailed"),
      )
    } else if (fresh?.installed && fresh.updateAvailable) {
      await run(
        () => AgentPlugin.Update(provider),
        t("settings.pluginSetting.updating", { name }),
        t("settings.pluginSetting.updated", { hint: RESTART_HINT }),
        t("settings.pluginSetting.updateFailed"),
      )
    }
    setBusy(false)
  }

  const check = async () => {
    setBusy(true)
    setChecked(false)
    await refresh()
    setChecked(true)
    setBusy(false)
  }

  const outcome =
    checked &&
    (error ? t("settings.pluginSetting.checkFailed") : t("settings.pluginSetting.checked"))

  const spinner = <LoaderCircle className="size-4 animate-spin" />
  const showTrustHint = statuses?.some((s) => s.provider === "codex" && s.installed)
  const showCrushHint = statuses?.some((s) => s.provider === "crush" && s.installed)
  const showOMPHint = statuses?.some((s) => s.provider === "omp" && s.installed)
  const showAntigravityHint = statuses?.some((s) => s.provider === "antigravity" && s.installed)
  const showCursorHint = statuses?.some((s) => s.provider === "cursor" && s.installed)

  return (
    <SettingBlock
      icon={<Puzzle className="size-4" />}
      title={t("settings.pluginSetting.title")}
      description={t("settings.pluginSetting.description")}
    >
      <div className="flex w-full flex-col gap-1">
        {statuses?.map((status) => (
          <div key={status.provider} className="flex items-center gap-2.5 py-1 text-sm">
            <ProviderIcon kind={status.provider} size={15} />
            <span>{status.name}</span>
            <span className="ml-auto text-xs text-muted-foreground">
              {!status.available
                ? t("settings.pluginSetting.cliMissing")
                : !status.installed
                  ? t("settings.pluginSetting.notInstalled")
                  : status.compatible
                    ? `v${status.installedVersion}`
                    : t("settings.pluginSetting.unsupported", { version: status.installedVersion })}
            </span>
            {status.available && !status.installed && (
              <Button
                size="sm"
                onClick={() => void act(status.provider, status.name)}
                disabled={busy}
              >
                {busy ? spinner : null}
                {t("settings.pluginSetting.install")}
              </Button>
            )}
            {status.installed && status.updateAvailable && (
              <Button
                size="sm"
                variant="outline"
                onClick={() => void act(status.provider, status.name)}
                disabled={busy}
              >
                {busy ? spinner : null}
                {status.compatible
                  ? t("settings.pluginSetting.updateTo", { version: status.latestVersion })
                  : t("settings.pluginSetting.installVersion", { version: status.latestVersion })}
              </Button>
            )}
          </div>
        ))}
        {showTrustHint && (
          <p className="text-xs text-muted-foreground">
            {t("settings.pluginSetting.hint", { provider: "Codex", hint: CODEX_TRUST_HINT })}
          </p>
        )}
        {showCrushHint && (
          <p className="text-xs text-muted-foreground">
            {t("settings.pluginSetting.hint", { provider: "Crush", hint: CRUSH_SCOPE_HINT })}
          </p>
        )}
        {showOMPHint && (
          <p className="text-xs text-muted-foreground">
            {t("settings.pluginSetting.hint", {
              provider: "oh-my-pi",
              hint: NO_APPROVAL_EVENT_HINT,
            })}
          </p>
        )}
        {showAntigravityHint && (
          <p className="text-xs text-muted-foreground">
            {t("settings.pluginSetting.hint", {
              provider: "Antigravity",
              hint: NO_APPROVAL_EVENT_HINT,
            })}
          </p>
        )}
        {showCursorHint && (
          <p className="text-xs text-muted-foreground">
            {t("settings.pluginSetting.hint", {
              provider: "Cursor CLI",
              hint: CURSOR_SHARED_PLUGIN_HINT,
            })}
          </p>
        )}
        <div className="flex items-center gap-3 pt-1">
          <Button size="sm" variant="outline" onClick={() => void check()} disabled={busy}>
            {t("settings.pluginSetting.checkForUpdates")}
          </Button>
          {outcome && <span className="text-xs text-muted-foreground">{outcome}</span>}
        </div>
      </div>
    </SettingBlock>
  )
}
