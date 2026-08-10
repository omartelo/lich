import { useEffect, useState } from "react"
import { Bug, Info, ScrollText } from "lucide-react"
import { Button } from "@/components/ui/button"
import { SettingBlock } from "./SettingBlock"
import { System } from "@/lib/rpc"
import { REPO_URL, bugReportUrl } from "@/lib/support-url"
import { runWithToast } from "@/lib/toast-async"
import type { Diagnostics } from "@/lib/api-types"

// The section a bug report starts from: the two things a reporter otherwise has
// to be walked through — where the log file lives, and what to put in the issue.
export function HelpSettings() {
  const [diagnostics, setDiagnostics] = useState<Diagnostics | null>(null)

  useEffect(() => {
    void System.Diagnostics()
      .then(setDiagnostics)
      .catch(() => {})
  }, [])

  return (
    <>
      <SettingBlock
        icon={<Bug className="size-4" />}
        title="Report a bug"
        description="Opens the bug form in your browser with the version and platform filled in. lich does not file the issue — you write it yourself. Run `lich rage` in a terminal to pack the log, the versions and what lich found on this machine into one archive to attach."
      >
        <Button
          size="sm"
          onClick={() => diagnostics && void System.OpenExternal(bugReportUrl(diagnostics))}
          disabled={!diagnostics}
        >
          Open bug report
        </Button>
      </SettingBlock>

      <SettingBlock
        icon={<ScrollText className="size-4" />}
        title="Log file"
        description="The log holds absolute paths, project and branch names, and your gh login — never a session token. Read it before you attach it to anything."
      >
        <div className="flex items-center gap-3">
          <Button
            size="sm"
            variant="outline"
            onClick={() =>
              void runWithToast(
                "Opening log folder…",
                System.RevealLog,
                "Log folder opened.",
                "Could not open the log folder",
              )
            }
            disabled={!diagnostics?.logPath}
          >
            Open log folder
          </Button>
          <code className="min-w-0 truncate font-mono text-xs text-muted-foreground">
            {diagnostics?.logPath || "Logging to stderr only — there is no log file."}
          </code>
        </div>
      </SettingBlock>

      <SettingBlock
        icon={<Info className="size-4" />}
        title="About"
        description={
          diagnostics ? `lich ${diagnostics.version} — ${diagnostics.platform}.` : "lich."
        }
      >
        <Button size="sm" variant="ghost" onClick={() => void System.OpenExternal(REPO_URL)}>
          Repository
        </Button>
      </SettingBlock>
    </>
  )
}
