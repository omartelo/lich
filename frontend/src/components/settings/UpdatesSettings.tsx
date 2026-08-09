import { useEffect, useState } from "react"
import { LoaderCircle, RefreshCw, Sparkles } from "lucide-react"
import { Button } from "@/components/ui/button"
import { SettingBlock } from "./SettingBlock"
import { PatchNotesDialog } from "@/components/PatchNotesDialog"
import { PluginSetting } from "./PluginSetting"
import { AgentPlugin, PatchNotes } from "@/lib/rpc"
import { runUpdateCheck } from "@/lib/update/update-check"
import type { PatchNotes as PatchNotesData, PluginStatus } from "@/lib/api-types"

export function UpdatesSettings() {
  const [notes, setNotes] = useState<PatchNotesData | null>(null)
  const [notesOpen, setNotesOpen] = useState(false)
  const [checking, setChecking] = useState(false)
  const [checkResult, setCheckResult] = useState("")
  const [plugin, setPlugin] = useState<PluginStatus[] | null>(null)

  useEffect(() => {
    void PatchNotes.Current()
      .then(setNotes)
      .catch(() => {})
    void refreshPlugin()
  }, [])

  const refreshPlugin = async () => {
    setPlugin(await AgentPlugin.Status())
  }

  const checkApp = async () => {
    setChecking(true)
    setCheckResult("")
    try {
      const status = await runUpdateCheck()
      setCheckResult(
        status.updateAvailable
          ? `lich ${status.latestVersion} is available — follow the prompt.`
          : "You're on the latest version.",
      )
    } catch {
      setCheckResult("Check failed — are you online?")
    } finally {
      setChecking(false)
    }
  }

  const spinner = <LoaderCircle className="size-4 animate-spin" />

  return (
    <>
      <SettingBlock
        icon={<RefreshCw className="size-4" />}
        title="Application"
        description={`lich ${notes ? `v${notes.version}` : ""} — checks for updates on startup and hourly.`}
      >
        <div className="flex items-center gap-3">
          <Button size="sm" onClick={() => void checkApp()} disabled={checking}>
            {checking ? spinner : null}
            Check for updates
          </Button>
          {checkResult && <span className="text-xs text-muted-foreground">{checkResult}</span>}
        </div>
      </SettingBlock>

      <SettingBlock
        icon={<Sparkles className="size-4" />}
        title="What's new"
        description={
          notes?.groups ? `Patch notes for v${notes.version}.` : "No patch notes for this build."
        }
      >
        <Button
          size="sm"
          variant="outline"
          onClick={() => setNotesOpen(true)}
          disabled={!notes?.groups}
        >
          View patch notes
        </Button>
        {notesOpen && notes && (
          <PatchNotesDialog notes={notes} onClose={() => setNotesOpen(false)} />
        )}
      </SettingBlock>

      <PluginSetting statuses={plugin} onRefresh={refreshPlugin} />
    </>
  )
}
