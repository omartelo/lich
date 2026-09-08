import { useCallback, useEffect, useMemo, useSyncExternalStore } from "react"
import { createKeyedStore } from "@/lib/keyed-store"
import { sandboxKey, sandboxLevel, type SandboxLevel } from "@/lib/providers-store"
import { Store, System } from "@/lib/rpc"
import { useKeyedStore } from "@/lib/use-keyed-store"

// The rung a provider is on in one project, as the sidebar reads it. It lives
// out here rather than in a card's own state because the card is what has to
// notice the rung moving: Settings takes the screen while the sidebar stays
// mounted (App.tsx), so a value read once at mount would never move again.
//
// null is "no rung to compare against", which covers three cases the card must
// treat alike: nothing read yet, a lookup that failed, and a machine with no
// sandbox backend at all. A stored "everywhere" on a machine that cannot
// confine would otherwise mark every card as one the sandbox missed.
const rungs = createKeyedStore<SandboxLevel | null>(null)

// Keys already asked for. Never cleared: a machine that cannot confine answers
// null forever, and re-reading it on every card mount would be a round trip per
// card for an answer that is not going to change.
const asked = new Set<string>()

// A shell is not a provider — the rung is keyed by provider and `shell` is not
// one, so a terminal session has no rung to disagree with.
function rungKey(providerId: string, projectId: string): string {
  return providerId && providerId !== "shell" ? `${providerId}\n${projectId}` : ""
}

function load(key: string): void {
  const [providerId, projectId] = key.split("\n")
  void Promise.all([
    System.SandboxBackend(),
    // The project's rung wins over the global one, the order store.SandboxLevel
    // reads in.
    projectId ? Store.GetSetting(sandboxKey(providerId), projectId) : Promise.resolve(""),
    Store.GetSetting(sandboxKey(providerId), ""),
  ])
    .then(([backend, scoped, global]) => {
      rungs.set(key, backend === "" ? null : sandboxLevel(scoped || global))
    })
    .catch(() => undefined)
}

/** Re-read every rung a card is comparing against. Called by the pane that
 * writes one: the whole point of the readout is that a card notices the ladder
 * moving, and nothing else on the settings screen reaches back here. */
export function refreshSandboxRungs(): void {
  for (const key of asked) {
    load(key)
  }
}

/** The rung this provider is on in this project, or null while there is nothing
 * to compare against (see the store above). */
export function useSandboxRung(providerId: string, projectId: string): SandboxLevel | null {
  const key = rungKey(providerId, projectId)
  useEffect(() => {
    if (!key || asked.has(key)) {
      return
    }
    asked.add(key)
    load(key)
  }, [key])
  return useKeyedStore(rungs, key)
}

/** The providers in this project whose rung is "Ask each time": the ones a menu
 * has to put the confinement question to before it opens a card, since that
 * rung has no answer of its own. Every other rung, and a machine that cannot
 * confine, is absent — the menu opens the card on the click as it always has.
 *
 * Reads the same store useSandboxRung does, so a card comparing a provider's
 * rung and a menu asking about it cost one round trip between them, and the
 * pane that writes a rung refreshes both. */
export function useSandboxAsk(
  providerIds: readonly string[],
  projectId: string,
): ReadonlySet<string> {
  // The array is rebuilt on every render of the menu; its contents are not.
  const ids = providerIds.join(" ")
  const keys = useMemo(
    () =>
      ids
        .split(" ")
        .filter(Boolean)
        .map((id) => rungKey(id, projectId))
        .filter(Boolean),
    [ids, projectId],
  )

  useEffect(() => {
    for (const key of keys) {
      if (!asked.has(key)) {
        asked.add(key)
        load(key)
      }
    }
  }, [keys])

  const subscribe = useCallback(
    (onChange: () => void) => {
      const offs = keys.map((key) => rungs.subscribe(key, onChange))
      return () => {
        for (const off of offs) {
          off()
        }
      }
    },
    [keys],
  )
  // Snapshotted as a joined string rather than a Set: useSyncExternalStore
  // compares snapshots by identity, and a collection rebuilt on every read is a
  // new one every render.
  const asking = useSyncExternalStore(subscribe, () =>
    keys
      .filter((key) => rungs.get(key) === "ask")
      .map((key) => key.split("\n")[0])
      .join(" "),
  )
  return useMemo(() => new Set(asking.split(" ").filter(Boolean)), [asking])
}
