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

function isProviderKind(id: string): id is ProviderKind {
  return (PROVIDER_KINDS as readonly string[]).includes(id)
}

export interface ProvidersDeps {
  detect: () => Promise<DetectedProvider[] | null>
  getEnabled: (id: string) => Promise<string>
  persistEnabled: (id: string, value: string) => void
}

// createProvidersStore builds a subscribable provider store over injected RPC.
export function createProvidersStore(deps: ProvidersDeps) {
  let providers: ProviderState[] = []
  let state: "idle" | "loading" | "ready" = "idle"
  const listeners = new Set<() => void>()
  const emit = () => listeners.forEach((listener) => listener())

  const load = async (): Promise<void> => {
    const detected = (await deps.detect()) ?? []
    providers = await Promise.all(
      detected.filter((d) => isProviderKind(d.id)).map(async (d) => ({
        id: d.id as ProviderKind,
        name: d.name,
        installed: d.installed,
        enabled: readEnabled(d.id, await deps.getEnabled(d.id)),
      })),
    )
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

  const subscribe = (listener: () => void): (() => void) => {
    listeners.add(listener)
    return () => {
      listeners.delete(listener)
    }
  }

  return { load, ensureLoaded, setEnabled, subscribe, getSnapshot: () => providers }
}

// The app-wide singleton, wired to the real RPC.
const store = createProvidersStore({
  detect: () => Providers.Detect(),
  getEnabled: (id) => Store.GetSetting(enabledKey(id), GLOBAL_SCOPE),
  persistEnabled: (id, value) => {
    void Store.SetSetting(enabledKey(id), GLOBAL_SCOPE, value)
  },
})

// setProviderEnabled flips a provider's global flag and persists it.
export function setProviderEnabled(id: ProviderKind, enabled: boolean): void {
  store.setEnabled(id, enabled)
}

// useProviders returns the known providers with their install + enabled state,
// loading them on first use.
export function useProviders(): ProviderState[] {
  const snapshot = useSyncExternalStore(store.subscribe, store.getSnapshot)
  useEffect(store.ensureLoaded, [])
  return snapshot
}
