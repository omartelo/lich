import { Bug, Info, ScrollText } from "lucide-react"
import { Button } from "@/components/ui/button"
import { useT } from "@/lib/i18n/i18n"
import { SettingBlock } from "./SettingBlock"
import { System } from "@/lib/rpc"
import { REPO_URL, bugReportUrl } from "@/lib/support-url"
import { runWithToast } from "@/lib/toast-async"
import { useRemoteResource } from "@/lib/use-remote-resource"
import type { Diagnostics } from "@/lib/api-types"

// The section a bug report starts from: the two things a reporter otherwise has
// to be walked through — where the log file lives, and what to put in the issue.
export function HelpSettings() {
  const t = useT()
  const { data: diagnostics } = useRemoteResource<Diagnostics | null>(
    "diagnostics",
    () => System.Diagnostics(),
    { empty: null, cache: "settings.diagnostics" },
  )

  return (
    <>
      <SettingBlock
        icon={<Bug className="size-4" />}
        title={t("settings.helpSettings.bugTitle")}
        description={t("settings.helpSettings.bugDescription")}
      >
        <Button
          size="sm"
          onClick={() => diagnostics && void System.OpenExternal(bugReportUrl(diagnostics))}
          disabled={!diagnostics}
        >
          {t("settings.helpSettings.openBugReport")}
        </Button>
      </SettingBlock>

      <SettingBlock
        icon={<ScrollText className="size-4" />}
        title={t("settings.helpSettings.logTitle")}
        description={t("settings.helpSettings.logDescription")}
      >
        <div className="flex items-center gap-3">
          <Button
            size="sm"
            variant="outline"
            onClick={() =>
              void runWithToast(
                t("settings.helpSettings.openingLogFolder"),
                System.RevealLog,
                t("settings.helpSettings.logFolderOpened"),
                t("settings.helpSettings.openLogFolderFailed"),
              )
            }
            disabled={!diagnostics?.logPath}
          >
            {t("settings.helpSettings.openLogFolder")}
          </Button>
          <code className="min-w-0 truncate font-mono text-xs text-muted-foreground">
            {diagnostics?.logPath || t("settings.helpSettings.noLogFile")}
          </code>
        </div>
      </SettingBlock>

      <SettingBlock
        icon={<Info className="size-4" />}
        title={t("settings.helpSettings.aboutTitle")}
        description={
          diagnostics
            ? t("settings.helpSettings.aboutVersion", {
                version: diagnostics.version,
                platform: diagnostics.platform,
              })
            : "lich."
        }
      >
        <Button size="sm" variant="ghost" onClick={() => void System.OpenExternal(REPO_URL)}>
          {t("settings.helpSettings.repository")}
        </Button>
      </SettingBlock>
    </>
  )
}
