import {useEffect, useState} from "react"
import {LoaderCircle, Puzzle, RefreshCw, Sparkles} from "lucide-react"
import {Button} from "@/components/ui/button"
import {SettingBlock} from "./SettingBlock"
import {PatchNotesDialog} from "@/components/PatchNotesDialog"
import {ClaudePlugin, PatchNotes} from "@/lib/rpc"
import {runUpdateCheck} from "@/lib/update-check"
import {runWithToast} from "@/lib/toast-async"
import type {PatchNotes as PatchNotesData, PluginStatus} from "@/lib/api-types"

export function UpdatesSettings() {
  const [notes, setNotes] = useState<PatchNotesData | null>(null)
  const [notesOpen, setNotesOpen] = useState(false)
  const [checking, setChecking] = useState(false)
  const [checkResult, setCheckResult] = useState("")
  const [plugin, setPlugin] = useState<PluginStatus | null>(null)
  const [pluginBusy, setPluginBusy] = useState(false)
  const [pluginResult, setPluginResult] = useState("")

  useEffect(() => {
    void PatchNotes.Current().then(setNotes).catch(() => {})
    void ClaudePlugin.Status().then(setPlugin).catch(() => {})
  }, [])

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

  const checkPlugin = async () => {
    setPluginBusy(true)
    setPluginResult("")
    try {
      const status = await ClaudePlugin.Status()
      setPlugin(status)
      if (status.installed && !status.updateAvailable) {
        setPluginResult("You're on the latest version.")
      }
    } catch {
      setPluginResult("Check failed — are you online?")
    } finally {
      setPluginBusy(false)
    }
  }

  const runPlugin = async (
    run: () => Promise<null>,
    progress: string,
    done: string,
    failed: string,
  ) => {
    setPluginBusy(true)
    if (await runWithToast(progress, run, done, failed)) {
      await ClaudePlugin.Status().then(setPlugin).catch(() => {})
    }
    setPluginBusy(false)
  }

  const spinner = <LoaderCircle className="size-4 animate-spin" />
  const restartHint = "restart your Claude sessions to apply."

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
        description={notes?.groups ? `Patch notes for v${notes.version}.` : "No patch notes for this build."}
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

      <SettingBlock
        icon={<Puzzle className="size-4" />}
        title="Claude Code plugin"
        description={
          plugin === null
            ? "The lich plugin for Claude Code."
            : plugin.installed
              ? `Installed: v${plugin.installedVersion}.`
              : "Not installed."
        }
      >
        <div className="flex items-center gap-3">
          {plugin !== null && !plugin.installed && (
            <Button
              size="sm"
              onClick={() =>
                void runPlugin(
                  ClaudePlugin.Install,
                  "Installing lich plugin…",
                  `Plugin installed — ${restartHint}`,
                  "Install failed",
                )
              }
              disabled={pluginBusy}
            >
              {pluginBusy ? spinner : null}
              Install
            </Button>
          )}
          {plugin?.installed && plugin.updateAvailable && (
            <Button
              size="sm"
              onClick={() =>
                void runPlugin(
                  ClaudePlugin.Update,
                  "Updating lich plugin…",
                  `Plugin updated — ${restartHint}`,
                  "Update failed",
                )
              }
              disabled={pluginBusy}
            >
              {pluginBusy ? spinner : null}
              Update to v{plugin.latestVersion}
            </Button>
          )}
          <Button
            size="sm"
            variant="outline"
            onClick={() => void checkPlugin()}
            disabled={pluginBusy}
          >
            Check for updates
          </Button>
          {pluginResult && <span className="text-xs text-muted-foreground">{pluginResult}</span>}
        </div>
      </SettingBlock>
    </>
  )
}
