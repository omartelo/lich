// Shared provider state: which harnesses are installed on the machine and which
// the user enabled. Both the New Session menu and the Settings screen read it,
// so a toggle in one place is reflected in the other without a refetch. Detection
// runs once (providers.Detect); enabled flags are global settings ("1"/"0").
//
// The store is a dependency-injected factory (its RPC calls are passed in) so it
// is testable without React or the network, mirroring git-status-store. A module
// singleton wires it to the real RPC, and useProviders is the React wrapper.
import { useEffect, useSyncExternalStore } from "react"
import type { DetectedProvider } from "./api-types"
import { Providers, Store } from "./rpc"
import { PROVIDER_KINDS, type ProviderKind } from "./sessions"

const GLOBAL_SCOPE = ""

// defaultKey holds the global id of the provider new sessions spawn by default
// (worktrees, the new-session hotkey, a project's first session). Empty resolves
// to the first enabled provider — Claude, until the user turns others on.
const defaultKey = "provider.default"

// enabledKey holds a provider's global enabled flag; binKey its custom binary
// path. Claude keeps the legacy "claude.bin" key (mirrors store.binKey in Go),
// so overrides set before the providers feature keep resolving.
export function enabledKey(id: string): string {
  return `provider.${id}.enabled`
}

export function binKey(id: string): string {
  return id === "claude" ? "claude.bin" : `provider.${id}.bin`
}

// readEnabled interprets the stored flag: Claude is enabled by default (it was
// always offered before the providers feature), every other provider is opt-in.
// An explicit "1"/"0" overrides the default.
export function readEnabled(id: string, value: string): boolean {
  if (value === "1") return true
  if (value === "0") return false
  return id === "claude"
}

export interface ProviderState {
  id: ProviderKind
  name: string
  installed: boolean
  enabled: boolean
}

// enabledProviders are the ones offered in New Session. Not filtered by install
// state on purpose: a Claude with a custom bin path (so "claude" is not on PATH)
// must still appear — a genuinely missing binary surfaces as a PTY error.
export function enabledProviders(list: ProviderState[]): ProviderState[] {
  return list.filter((p) => p.enabled)
}

// resolveDefaultProvider picks the kind a new session spawns: the stored default
// if it still names an enabled provider, else the first enabled one, else Claude
// (nothing loaded yet, or every provider off). Mirrors enabledProviders in
// ignoring install state — a disabled default falls back, a missing binary does
// not, that shows up as a PTY error.
export function resolveDefaultProvider(
  list: ProviderState[],
  defaultId: string,
): ProviderKind {
  const enabled = enabledProviders(list)
  const chosen = enabled.find((p) => p.id === defaultId)
  return chosen?.id ?? enabled[0]?.id ?? "claude"
}

function isProviderKind(id: string): id is ProviderKind {
  return (PROVIDER_KINDS as readonly string[]).includes(id)
}

export interface ProvidersDeps {
  detect: () => Promise<DetectedProvider[] | null>
  getEnabled: (id: string) => Promise<string>
  persistEnabled: (id: string, value: string) => void
  getDefault: () => Promise<string>
  persistDefault: (id: string) => void
}

export function createProvidersStore(deps: ProvidersDeps) {
  let providers: ProviderState[] = []
  let defaultId = ""
  let state: "idle" | "loading" | "ready" = "idle"
  const listeners = new Set<() => void>()
  const emit = () => listeners.forEach((listener) => listener())

  const load = async (): Promise<void> => {
    const [detected, storedDefault] = await Promise.all([
      deps.detect().then((d) => d ?? []),
      deps.getDefault(),
    ])
    providers = await Promise.all(
      detected.filter((d) => isProviderKind(d.id)).map(async (d) => ({
        id: d.id as ProviderKind,
        name: d.name,
        installed: d.installed,
        enabled: readEnabled(d.id, await deps.getEnabled(d.id)),
      })),
    )
    defaultId = storedDefault
    state = "ready"
    emit()
  }

  // ensureLoaded runs load once; a failed attempt resets so a later mount retries.
  const ensureLoaded = (): void => {
    if (state !== "idle") {
      return
    }
    state = "loading"
    void load().catch(() => {
      state = "idle"
    })
  }

  const setEnabled = (id: ProviderKind, enabled: boolean): void => {
    providers = providers.map((p) => (p.id === id ? { ...p, enabled } : p))
    emit()
    deps.persistEnabled(id, enabled ? "1" : "0")
  }

  const setDefault = (id: ProviderKind): void => {
    defaultId = id
    emit()
    deps.persistDefault(id)
  }

  const subscribe = (listener: () => void): (() => void) => {
    listeners.add(listener)
    return () => {
      listeners.delete(listener)
    }
  }

  return {
    load,
    ensureLoaded,
    setEnabled,
    setDefault,
    subscribe,
    getSnapshot: () => providers,
    getDefaultSnapshot: () => defaultId,
  }
}

const store = createProvidersStore({
  detect: () => Providers.Detect(),
  getEnabled: (id) => Store.GetSetting(enabledKey(id), GLOBAL_SCOPE),
  persistEnabled: (id, value) => {
    void Store.SetSetting(enabledKey(id), GLOBAL_SCOPE, value)
  },
  getDefault: () => Store.GetSetting(defaultKey, GLOBAL_SCOPE),
  persistDefault: (id) => {
    void Store.SetSetting(defaultKey, GLOBAL_SCOPE, id)
  },
})

export function setProviderEnabled(id: ProviderKind, enabled: boolean): void {
  store.setEnabled(id, enabled)
}

export function setProviderDefault(id: ProviderKind): void {
  store.setDefault(id)
}

// defaultProviderKind resolves the current default synchronously, for the
// imperative session-spawn call sites that cannot use a hook. Before the store
// loads it resolves to Claude (empty list, empty default).
export function defaultProviderKind(): ProviderKind {
  return resolveDefaultProvider(store.getSnapshot(), store.getDefaultSnapshot())
}

// useProviders returns the known providers with their install + enabled state,
// loading them on first use.
export function useProviders(): ProviderState[] {
  const snapshot = useSyncExternalStore(store.subscribe, store.getSnapshot)
  useEffect(store.ensureLoaded, [])
  return snapshot
}

// useDefaultProvider returns the resolved default provider kind, tracking both
// the stored default and enable changes (a disabled default falls back live).
export function useDefaultProvider(): ProviderKind {
  const providers = useSyncExternalStore(store.subscribe, store.getSnapshot)
  const defaultId = useSyncExternalStore(store.subscribe, store.getDefaultSnapshot)
  useEffect(store.ensureLoaded, [])
  return resolveDefaultProvider(providers, defaultId)
}
