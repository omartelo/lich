import { useState } from "react"
import { LoaderCircle, RefreshCw, Sparkles } from "lucide-react"
import { Button } from "@/components/ui/button"
import { SettingBlock } from "./SettingBlock"
import { PatchNotesDialog } from "@/components/PatchNotesDialog"
import { PluginSetting } from "./PluginSetting"
import { useT } from "@/lib/i18n/i18n"
import { PatchNotes } from "@/lib/rpc"
import { runUpdateCheck } from "@/lib/update/update-check"
import { useRemoteResource } from "@/lib/use-remote-resource"
import type { PatchNotes as PatchNotesData } from "@/lib/api-types"

export function UpdatesSettings() {
  const t = useT()
  const [notesOpen, setNotesOpen] = useState(false)
  const [checking, setChecking] = useState(false)
  const [checkResult, setCheckResult] = useState("")
  const { data: notes } = useRemoteResource<PatchNotesData | null>(
    "patch-notes",
    () => PatchNotes.Current(),
    { empty: null, cache: "settings.patchNotes" },
  )
  const checkApp = async () => {
    setChecking(true)
    setCheckResult("")
    try {
      const status = await runUpdateCheck()
      setCheckResult(
        status.updateAvailable
          ? t("settings.updatesSettings.available", { version: status.latestVersion })
          : t("settings.updatesSettings.upToDate"),
      )
    } catch {
      setCheckResult(t("settings.updatesSettings.checkFailed"))
    } finally {
      setChecking(false)
    }
  }

  const spinner = <LoaderCircle className="size-4 animate-spin" />

  return (
    <>
      <SettingBlock
        icon={<RefreshCw className="size-4" />}
        title={t("settings.updatesSettings.applicationTitle")}
        description={t("settings.updatesSettings.applicationDescription", {
          version: notes ? `v${notes.version}` : "",
        })}
      >
        <div className="flex items-center gap-3">
          <Button size="sm" onClick={() => void checkApp()} disabled={checking}>
            {checking ? spinner : null}
            {t("settings.updatesSettings.checkForUpdates")}
          </Button>
          {checkResult && <span className="text-xs text-muted-foreground">{checkResult}</span>}
        </div>
      </SettingBlock>

      <SettingBlock
        icon={<Sparkles className="size-4" />}
        title={t("settings.updatesSettings.whatsNewTitle")}
        description={
          notes?.groups
            ? t("settings.updatesSettings.patchNotesFor", { version: notes.version })
            : t("settings.updatesSettings.noPatchNotes")
        }
      >
        <Button
          size="sm"
          variant="outline"
          onClick={() => setNotesOpen(true)}
          disabled={!notes?.groups}
        >
          {t("settings.updatesSettings.viewPatchNotes")}
        </Button>
        {notesOpen && notes && (
          <PatchNotesDialog notes={notes} onClose={() => setNotesOpen(false)} />
        )}
      </SettingBlock>

      <PluginSetting />
    </>
  )
}
