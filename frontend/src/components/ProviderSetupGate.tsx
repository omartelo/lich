import { useEffect, useRef, useState } from "react"
import { ProviderSetupDialog } from "./ProviderSetupDialog"
import { decideProviderSetup } from "@/lib/provider-setup"
import {
  setProviderDefault,
  setProviderEnabled,
  useDefaultProvider,
  useProviders,
  useStoredDefaultProvider,
} from "@/lib/providers-store"

// ProviderSetupGate opens the provider dialog once, on the first launch that has
// no chosen default. Detection runs first, so a user who installed only Codex is
// asked about Codex — lich no longer assumes the harness its author happens to
// run.
export function ProviderSetupGate() {
  const providers = useProviders()
  const storedDefault = useStoredDefaultProvider()
  const resolvedDefault = useDefaultProvider()
  // Set at open and never recomputed: the dialog's shape must not change under
  // the user, and every toggle inside it moves the state the decision reads.
  const [hasInstalled, setHasInstalled] = useState<boolean | null>(null)
  const opened = useRef(false)

  const decision = decideProviderSetup(providers, storedDefault)
  const show = decision.kind === "show"
  const preselect = decision.kind === "show" ? decision.preselect : null

  useEffect(() => {
    if (opened.current || !show) {
      return
    }
    // The ref is the latch, not the state below: setProviderEnabled emits
    // synchronously, so useSyncExternalStore re-enters this effect before any
    // setState has landed. A state guard would recurse until React gives up.
    opened.current = true
    // Claude is enabled by default even when it is not on PATH (readEnabled), so
    // on a Codex-only machine it would sit outside this list and still be offered
    // in New Session. Writing the absent ones off is what makes the list the
    // whole truth. Skipped when nothing is installed: there is no list to be true
    // to, and turning everything off would leave the user no provider at all.
    if (preselect) {
      for (const provider of providers) {
        if (!provider.installed && provider.enabled) {
          setProviderEnabled(provider.id, false)
        }
      }
      setProviderEnabled(preselect, true)
    }
    setHasInstalled(preselect !== null)
  }, [show, preselect, providers])

  if (hasInstalled === null) {
    return null
  }

  const finish = () => {
    setProviderDefault(resolvedDefault)
    setHasInstalled(null)
  }

  return (
    <ProviderSetupDialog
      hasInstalled={hasInstalled}
      knownNames={providers.map((provider) => provider.name)}
      onDone={finish}
    />
  )
}
