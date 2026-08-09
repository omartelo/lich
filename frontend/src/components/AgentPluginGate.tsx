import { useEffect, useRef, useState } from "react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { ProviderIcon } from "@/components/ProviderIcon"
import {
  CODEX_TRUST_HINT,
  decidePluginAction,
  DISMISSED_FLAG,
  INSTALL_DISMISSED_KEY,
  type PluginAction,
  RESTART_HINT,
  type Status,
  UPDATE_DISMISSED_KEY,
} from "@/lib/update/plugin-gate"
import { AgentPlugin } from "@/lib/rpc"
import { enabledProviders, useProviders, useStoredDefaultProvider } from "@/lib/providers-store"
import { errorText } from "@/lib/utils"
import { readPref, writePref } from "@/lib/prefs"
import { runWithToast } from "@/lib/toast-async"

// AgentPluginGate checks on startup whether the lich plugin is installed and
// current in the provider CLIs the user turned on. Missing → an install modal
// listing those CLIs, ticked by default; a newer release → a non-blocking,
// actionable update toast. Plugin hooks only load in new sessions, so both
// actions remind the user to restart. Any failure is silent — it must never
// block or break startup.
//
// It waits for a stored default provider, which is what ProviderSetupGate writes
// on a first launch: asking to install a plugin before the user has said which
// harnesses they use would stack two modals and offer one they are about to turn
// off.
export function AgentPluginGate() {
  const [offer, setOffer] = useState<Status[]>([])
  const [picked, setPicked] = useState<string[]>([])
  const [installing, setInstalling] = useState(false)
  const providers = useProviders()
  const storedDefault = useStoredDefaultProvider()
  // Guard React strict-mode's double effect, and the re-renders the provider
  // store emits: the check runs once per start.
  const checked = useRef(false)

  useEffect(() => {
    if (checked.current || storedDefault === "") return
    checked.current = true
    void check(enabledProviders(providers).map((p) => p.id))
  }, [providers, storedDefault])

  const check = async (enabled: string[]) => {
    let action: PluginAction
    try {
      const statuses = await AgentPlugin.Status()
      action = decidePluginAction(
        statuses.filter((s) => enabled.includes(s.provider)),
        readPref(INSTALL_DISMISSED_KEY) === DISMISSED_FLAG,
        readPref(UPDATE_DISMISSED_KEY),
      )
    } catch {
      return
    }
    if (action.kind === "install") {
      setOffer(action.providers)
      setPicked(action.providers.map((p) => p.provider))
    } else if (action.kind === "update") {
      promptUpdate(action.version, action.providers)
    }
  }

  const promptUpdate = (version: string, providers: Status[]) => {
    toast(`lich plugin ${version} is available`, {
      duration: Infinity,
      action: { label: "Update", onClick: () => void runUpdate(providers) },
      cancel: {
        label: "Later",
        onClick: () => writePref(UPDATE_DISMISSED_KEY, version),
      },
    })
  }

  const runUpdate = (providers: Status[]) =>
    runWithToast(
      "Updating lich plugin…",
      () =>
        installAll(
          providers.map((p) => p.provider),
          AgentPlugin.Update,
        ),
      `Plugin updated — ${RESTART_HINT}`,
      "Update failed",
    )

  const close = () => setOffer([])

  const runInstall = async () => {
    setInstalling(true)
    try {
      await installAll(picked, AgentPlugin.Install)
      close()
      toast.success(`Plugin installed — ${RESTART_HINT}`)
      if (picked.includes("codex")) {
        toast.info(`Codex: ${CODEX_TRUST_HINT}`, { duration: Infinity })
      }
    } catch (error) {
      toast.error(`Install failed: ${errorText(error)}`)
    } finally {
      setInstalling(false)
    }
  }

  const dismissForever = () => {
    writePref(INSTALL_DISMISSED_KEY, DISMISSED_FLAG)
    close()
  }

  const toggle = (provider: string) =>
    setPicked((prev) =>
      prev.includes(provider) ? prev.filter((p) => p !== provider) : [...prev, provider],
    )

  return (
    <Dialog open={offer.length > 0} onOpenChange={(open) => !open && !installing && close()}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Enable the agent integration</DialogTitle>
          <DialogDescription>
            The lich plugin reports your sessions' status and titles to the app, refreshes their git
            status, and lets a restored session resume its conversation. Install it into the CLIs
            you use.
          </DialogDescription>
        </DialogHeader>
        <div className="flex flex-col">
          {offer.map((status) => (
            <label
              key={status.provider}
              htmlFor={`plugin-${status.provider}`}
              className="flex cursor-pointer items-center gap-2.5 rounded-md px-2 py-2 text-sm hover:bg-accent/50"
            >
              <Checkbox
                id={`plugin-${status.provider}`}
                checked={picked.includes(status.provider)}
                onCheckedChange={() => toggle(status.provider)}
                disabled={installing}
              />
              <ProviderIcon kind={status.provider} size={15} />
              <span className="font-medium">{status.name}</span>
              {status.provider === "codex" && (
                <span className="ml-auto text-xs text-muted-foreground">
                  needs /hooks to trust it
                </span>
              )}
            </label>
          ))}
        </div>
        <DialogFooter>
          <Button variant="ghost" onClick={dismissForever} disabled={installing}>
            Don't ask again
          </Button>
          <Button variant="ghost" onClick={close} disabled={installing}>
            Not now
          </Button>
          <Button onClick={() => void runInstall()} disabled={installing || picked.length === 0}>
            {installing ? "Installing…" : "Install"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// installAll runs one plugin call per provider, in order, and stops at the first
// failure so the error names the CLI that refused rather than the last one tried.
async function installAll(providers: string[], run: (provider: string) => Promise<null>) {
  for (const provider of providers) {
    await run(provider)
  }
  return null
}
