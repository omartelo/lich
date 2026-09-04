// Shared provider state: which providers are installed on the machine and which
// the user enabled. Both the New Session menu and the Settings screen read it,
// so a toggle in one place is reflected in the other without a refetch. Detection
// runs on first use and again on demand (providers.Detect, refresh); enabled
// flags are global settings ("1"/"0").
//
// The store is a dependency-injected factory (its RPC calls are passed in) so it
// is testable without React or the network, mirroring git-status-store. A module
// singleton wires it to the real RPC, and useProviders is the React wrapper.
import { useCallback, useEffect, useSyncExternalStore } from "react"
import type { DetectedProvider } from "./api-types"
import { Providers, Store } from "./rpc"
import { PROVIDER_KINDS, type ProviderKind, type SessionKind } from "@/lib/session/sessions"

const GLOBAL_SCOPE = ""

// defaultKey holds the id of the provider new sessions spawn by default
// (worktrees, the new-session hotkey, a project's first session), in either
// scope: global, or one project overriding it. Empty resolves to the first
// enabled provider — Claude, until the user turns others on. Mirrors
// store.providerDefaultKey in Go, which is what LoadState hydrates from.
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

// binOffKey parks a binary layer without erasing it: the path stays in the
// store, the layer stops resolving. Mirrors store.binOffKey in Go, scoped
// exactly like the path it parks.
export function binOffKey(id: string): string {
  return `${binKey(id)}.off`
}

// skipPermissionsKey holds the flag that spawns a provider without its
// permission prompts, in one of two scopes (mirrors store.skipPermissionsKey in
// Go, which is what the spawn reads). Two keys because a worktree is a throwaway
// checkout: letting an agent loose there while the main working tree keeps
// asking is a position a single flag cannot express.
export function skipPermissionsKey(id: string, worktree: boolean): string {
  return `provider.${id}.skip-permissions${worktree ? ".worktree" : ""}`
}

// skipPermissionFlags mirrors terminal.skipPermissionFlags in Go: how each
// provider spells "run every tool without asking". Settings offers the switch
// only for a provider listed here, so the UI cannot promise a flag the spawn
// has no spelling for.
export const skipPermissionFlags: Record<string, string> = {
  claude: "--dangerously-skip-permissions",
  codex: "--dangerously-bypass-approvals-and-sandbox",
  antigravity: "--dangerously-skip-permissions",
  opencode: "--auto",
  omp: "--auto-approve",
  crush: "--yolo",
  cursor: "--force",
  kiro: "--trust-all-tools",
}

// How far a provider runs without asking, as one ladder ordered by risk. The
// two settings keys stay exactly as they are — this is the shape the user
// chooses in, not the shape lich stores.
export type SkipLevel = "never" | "worktrees" | "everywhere"

// skipLevel folds the stored pair into a rung. The fourth combination — skip in
// the working tree, ask in worktrees — is the inversion of the whole argument
// for the setting, and reads as "everywhere": it is what that pair already did
// to the checkout the user works in.
export function skipLevel(here: boolean, worktrees: boolean): SkipLevel {
  if (here) {
    return "everywhere"
  }
  return worktrees ? "worktrees" : "never"
}

// skipLevelPair is the inverse: the pair a chosen rung writes back. Composing
// it with skipLevel is the identity on all three rungs, which is what keeps a
// round trip through the control from moving a setting the user did not touch.
export function skipLevelPair(level: SkipLevel): { here: boolean; worktrees: boolean } {
  return { here: level === "everywhere", worktrees: level !== "never" }
}

// The skip ladder read as exposure, safest first — which is the order it is
// already declared in. Both standing safety settings are gated against their own
// order by climbsToRiskier below.
export const SKIP_RISK_ORDER: readonly SkipLevel[] = ["never", "worktrees", "everywhere"]

// climbsToRiskier answers whether a click moves toward more exposure, the only
// direction a standing safety setting asks about: turning an automation off must
// never be the harder direction. Re-clicking the chosen rung is not a move and
// answers false, so nothing is confirmed that changes nothing.
export function climbsToRiskier<T extends string>(
  order: readonly T[],
  current: T,
  next: T,
): boolean {
  return order.indexOf(next) > order.indexOf(current)
}

// sandboxKey holds which sessions of a provider run confined (mirrors
// store.sandboxKey in Go, which is what the spawn reads). Scoped like binKey —
// a project value wins over the global one — because the checkout full of
// somebody else's code is the one to confine, and a scratch project is not.
export function sandboxKey(id: string): string {
  return `provider.${id}.sandbox`
}

// The sandbox grant keys, each handing a confined session one credential its
// private home took away: the ssh agent a push authenticates through, and the
// GitHub token gh works through (mirror store.sshAgentKey and store.ghTokenKey
// in Go, which are what the spawn reads).
//
// Two keys rather than one because they open different doors — the agent signs
// with every identity loaded into it, for any host; the token is one account's,
// with that account's scopes. Neither carries a provider: a grant describes what
// exists inside the sandbox, not which agent runs in it. Scoped like sandboxKey,
// stored as "true" or nothing at all.
export const SSH_AGENT_KEY = "sandbox.ssh-agent"
export const GH_TOKEN_KEY = "sandbox.gh-token"

// Which sessions of a provider run confined, as one ladder ordered by how much
// of the machine a session can reach. "ask" sits second because a session
// nobody answered for runs unconfined, exactly like "off" — it moves who
// decides, not what the sandbox is. Mirrors the Sandbox* constants in Go.
export type SandboxLevel = "off" | "ask" | "worktrees" | "everywhere"

const SANDBOX_LEVELS: readonly SandboxLevel[] = ["off", "ask", "worktrees", "everywhere"]

// sandboxLevel reads the stored value. Anything the ladder does not name is
// "off": an unknown value must never confine a session nobody asked to confine,
// nor leave one unconfined that the user meant to protect — and "off" is the
// only answer that surprises no one, because it is what lich did before.
export function sandboxLevel(value: string): SandboxLevel {
  return SANDBOX_LEVELS.find((level) => level === value) ?? "off"
}

// SANDBOX_LEVELS read the other way round: exposure is the inverse of
// confinement, so the safest rung is "everywhere" and the riskiest is "off".
// Derived rather than written out twice — a rung added to the ladder is a rung
// the gate already knows the risk of.
export const SANDBOX_RISK_ORDER: readonly SandboxLevel[] = [...SANDBOX_LEVELS].reverse()

// sandboxDefaultFor is what the new-session dialog arrives showing: the rung
// applied to the checkout the session will start in. It is the frontend's copy
// of store.SandboxDefault in Go — the dialog has to show the same answer the
// spawn would reach on its own, or the box the user leaves untouched means
// something other than what it shows.
export function sandboxDefaultFor(level: SandboxLevel, worktree: boolean): boolean {
  if (level === "everywhere") return true
  return level === "worktrees" && worktree
}

// How much of a session's own accounting the footer carries, as one ladder
// ordered by how much it says. The two settings keys stay exactly as they are —
// this is the shape the user chooses in, not the shape lich stores.
export type FooterReadout = "off" | "context" | "cost"

// footerReadout folds the stored pair into a rung. The fourth combination — the
// cost without the model and the ring — is a readout nobody chose: the cost
// setting was always the extra one, offered beside a readout that was already
// there. It reads as the top rung, and choosing that rung back turns both on.
export function footerReadout(context: boolean, cost: boolean): FooterReadout {
  if (cost) {
    return "cost"
  }
  return context ? "context" : "off"
}

// footerReadoutPair is the inverse: the pair a chosen rung writes back.
export function footerReadoutPair(level: FooterReadout): { context: boolean; cost: boolean } {
  return { context: level !== "off", cost: level === "cost" }
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
  /** The executable a session spawns — a provider id is not its command. */
  binary: string
  installed: boolean
  enabled: boolean
  /** The page documenting how to install this CLI — offered on the rows that
   * found nothing, which are the only rows that need it. */
  docs: string
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
export function resolveDefaultProvider(list: ProviderState[], defaultId: string): ProviderKind {
  const enabled = enabledProviders(list)
  const chosen = enabled.find((p) => p.id === defaultId)
  return chosen?.id ?? enabled[0]?.id ?? "claude"
}

// resolveProjectDefaultProvider gives an enabled project override precedence,
// then delegates every fallback rule to the global resolver. Keeping that
// resolver as the one fallback path means disabling a provider has identical
// semantics in both scopes.
export function resolveProjectDefaultProvider(
  list: ProviderState[],
  globalDefaultId: string,
  projectDefaultId: string,
): ProviderKind {
  const chosen = enabledProviders(list).find((provider) => provider.id === projectDefaultId)
  return chosen?.id ?? resolveDefaultProvider(list, globalDefaultId)
}

// noProviderInstalled reports the one machine state in which spawning the
// resolved default is guaranteed to fail: detection has answered and found no
// binary at all. An empty list is detection not having answered yet — never
// this, exactly as in decideProviderSetup.
export function noProviderInstalled(list: ProviderState[]): boolean {
  return list.length > 0 && !list.some((provider) => provider.installed)
}

// resolveImplicitSessionKind is what a session nobody picked a kind for spawns:
// the empty screen's button, the new-session hotkey, a new worktree. It is
// resolveProjectDefaultProvider with one gate in front, because on a machine
// with no agent installed every fallback below still lands on Claude — readEnabled
// leaves Claude enabled by default, so the card opens on `claude: command not
// found`. A shell spawns with zero providers, and it is where the install command
// from the provider's docs link gets pasted.
export function resolveImplicitSessionKind(
  list: ProviderState[],
  globalDefaultId: string,
  projectDefaultId: string,
): SessionKind {
  if (noProviderInstalled(list)) {
    return "shell"
  }
  return resolveProjectDefaultProvider(list, globalDefaultId, projectDefaultId)
}

function isProviderKind(id: string): id is ProviderKind {
  return (PROVIDER_KINDS as readonly string[]).includes(id)
}

// A provider the backend knows and this build does not is dropped rather than
// carried as a kind nothing can spawn.
function isDetectedKind(provider: DetectedProvider): boolean {
  return isProviderKind(provider.id)
}

export interface ProvidersDeps {
  detect: () => Promise<DetectedProvider[] | null>
  getEnabled: (id: string) => Promise<string>
  persistEnabled: (id: string, value: string) => void
  getDefault: () => Promise<string>
  persistDefault: (id: string) => void
  persistProjectDefault: (projectId: string, id: string) => void
}

export interface HydratedProjectDefault {
  id: string
  defaultProvider: string
}

export interface ProvidersStore {
  load: () => Promise<void>
  ensureLoaded: () => void
  refresh: () => Promise<void>
  setEnabled: (id: ProviderKind, enabled: boolean) => void
  setDefault: (id: ProviderKind) => void
  hydrateProjectDefaults: (projects: readonly HydratedProjectDefault[]) => void
  setProjectDefault: (projectId: string, id: ProviderKind | "") => void
  subscribe: (listener: () => void) => () => void
  getSnapshot: () => ProviderState[]
  getDefaultSnapshot: () => string
  getProjectDefaultSnapshot: (projectId: string) => string
  getProjectProviderKind: (projectId: string) => ProviderKind
  getProjectSessionKind: (projectId: string) => SessionKind
}

class ProviderStoreImpl implements ProvidersStore {
  private providers: ProviderState[] = []
  private defaultId = ""
  private projectDefaultIds = new Map<string, string>()
  private state: "idle" | "loading" | "ready" = "idle"
  private providerLoad: Promise<void> | null = null
  private listeners = new Set<() => void>()

  constructor(private deps: ProvidersDeps) {}

  load = (): Promise<void> => {
    if (this.state === "ready") {
      return Promise.resolve()
    }
    if (this.providerLoad) {
      return this.providerLoad
    }
    this.state = "loading"
    this.providerLoad = this.performLoad()
      .catch((error: unknown) => {
        this.state = "idle"
        throw error
      })
      .finally(() => {
        this.providerLoad = null
      })
    return this.providerLoad
  }

  // ensureLoaded runs load once; a failed attempt resets so a later mount retries.
  ensureLoaded = (): void => {
    void this.load().catch(() => undefined)
  }

  // refresh re-probes PATH and nothing else: the enabled flags and the default
  // stay as they are in memory, so a toggle the user just made cannot be undone
  // by a settings read racing its own write. What it cannot see is a provider
  // installed outside the PATH lich resolved at boot (docs/ceilings.md).
  refresh = async (): Promise<void> => {
    const detected = (await this.deps.detect()) ?? []
    const enabled = new Map(this.providers.map((provider) => [provider.id, provider.enabled]))
    this.providers = detected.filter(isDetectedKind).map((provider) => ({
      id: provider.id as ProviderKind,
      name: provider.name,
      binary: provider.binary,
      installed: provider.installed,
      docs: provider.docs,
      enabled: enabled.get(provider.id as ProviderKind) ?? readEnabled(provider.id, ""),
    }))
    this.emit()
  }

  setEnabled = (id: ProviderKind, enabled: boolean): void => {
    this.providers = this.providers.map((p) => (p.id === id ? { ...p, enabled } : p))
    this.emit()
    this.deps.persistEnabled(id, enabled ? "1" : "0")
  }

  setDefault = (id: ProviderKind): void => {
    this.defaultId = id
    this.emit()
    this.deps.persistDefault(id)
  }

  hydrateProjectDefaults = (projects: readonly HydratedProjectDefault[]): void => {
    for (const project of projects) {
      this.projectDefaultIds.set(project.id, project.defaultProvider)
    }
    this.emit()
  }

  setProjectDefault = (projectId: string, id: ProviderKind | ""): void => {
    this.projectDefaultIds.set(projectId, id)
    this.emit()
    this.deps.persistProjectDefault(projectId, id)
  }

  subscribe = (listener: () => void): (() => void) => {
    this.listeners.add(listener)
    return () => {
      this.listeners.delete(listener)
    }
  }

  getSnapshot = (): ProviderState[] => this.providers
  getDefaultSnapshot = (): string => this.defaultId
  getProjectDefaultSnapshot = (projectId: string): string =>
    this.projectDefaultIds.get(projectId) ?? ""
  getProjectProviderKind = (projectId: string): ProviderKind =>
    resolveProjectDefaultProvider(
      this.providers,
      this.defaultId,
      this.getProjectDefaultSnapshot(projectId),
    )
  getProjectSessionKind = (projectId: string): SessionKind =>
    resolveImplicitSessionKind(
      this.providers,
      this.defaultId,
      this.getProjectDefaultSnapshot(projectId),
    )

  private async performLoad(): Promise<void> {
    const [detected, storedDefault] = await Promise.all([
      this.deps.detect().then((result) => result ?? []),
      this.deps.getDefault(),
    ])
    this.providers = await Promise.all(
      detected.filter(isDetectedKind).map(async (provider) => ({
        id: provider.id as ProviderKind,
        name: provider.name,
        binary: provider.binary,
        installed: provider.installed,
        docs: provider.docs,
        enabled: readEnabled(provider.id, await this.deps.getEnabled(provider.id)),
      })),
    )
    this.defaultId = storedDefault
    this.state = "ready"
    this.emit()
  }

  private emit(): void {
    for (const listener of this.listeners) {
      listener()
    }
  }
}

export function createProvidersStore(deps: ProvidersDeps): ProvidersStore {
  return new ProviderStoreImpl(deps)
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
  persistProjectDefault: (projectId, id) => {
    void Store.SetSetting(defaultKey, projectId, id)
  },
})

export function setProviderEnabled(id: ProviderKind, enabled: boolean): void {
  store.setEnabled(id, enabled)
}

export function setProviderDefault(id: ProviderKind): void {
  store.setDefault(id)
}

export function loadProviders(): Promise<void> {
  return store.load()
}

// refreshProviders re-probes PATH for the surfaces that offer it: the first-run
// dialog and Settings › Providers. It lives here rather than in either caller so
// both get the same answer without either owning the scan.
export function refreshProviders(): Promise<void> {
  return store.refresh()
}

// hydrateProjectProviderDefaults accepts the overrides delivered with workspace
// state. It is synchronous so restoring projects never waits on provider
// detection or performs one settings request per project.
export function hydrateProjectProviderDefaults(projects: readonly HydratedProjectDefault[]): void {
  store.hydrateProjectDefaults(projects)
}

export function setProjectProviderDefault(projectId: string, id: ProviderKind | ""): void {
  store.setProjectDefault(projectId, id)
}

// projectDefaultProviderKind is the imperative counterpart used by workspace
// mutations. Empty project state follows the global default dynamically; a
// disabled override delegates to the same global fallback rules.
export function projectDefaultProviderKind(projectId: string): ProviderKind {
  return store.getProjectProviderKind(projectId)
}

// projectNewSessionKind is what an implicit new session spawns in this project.
// Every implicit entry point routes through it — the empty screen, the hotkey, a
// new worktree — so a machine with no agent installed opens a shell everywhere
// rather than in whichever caller remembered to check.
export function projectNewSessionKind(projectId: string): SessionKind {
  return store.getProjectSessionKind(projectId)
}

// useProviders returns the known providers with their install + enabled state,
// loading them on first use.
export function useProviders(): ProviderState[] {
  const snapshot = useSyncExternalStore(store.subscribe, store.getSnapshot)
  useEffect(store.ensureLoaded, [])
  return snapshot
}

// useStoredDefaultProvider returns the persisted default id unresolved — "" when
// the user has never chosen one. Only the first-run gate wants that: every other
// caller wants useDefaultProvider, which resolves the empty value away and so can
// never report the absence.
export function useStoredDefaultProvider(): string {
  const defaultId = useSyncExternalStore(store.subscribe, store.getDefaultSnapshot)
  useEffect(store.ensureLoaded, [])
  return defaultId
}

// useDefaultProvider returns the resolved default provider kind, tracking both
// the stored default and enable changes (a disabled default falls back live).
export function useDefaultProvider(): ProviderKind {
  const providers = useSyncExternalStore(store.subscribe, store.getSnapshot)
  const defaultId = useSyncExternalStore(store.subscribe, store.getDefaultSnapshot)
  useEffect(store.ensureLoaded, [])
  return resolveDefaultProvider(providers, defaultId)
}

// useNoProviderInstalled reports the machine with no agent on PATH, which is
// what a surface offering an implicit session shows a terminal for.
export function useNoProviderInstalled(): boolean {
  const providers = useSyncExternalStore(store.subscribe, store.getSnapshot)
  useEffect(store.ensureLoaded, [])
  return noProviderInstalled(providers)
}

// useStoredProjectDefaultProvider returns the unresolved project value. Empty
// means the project follows the global default; the Providers settings pane
// needs that distinction to clear and disable its Use default action.
export function useStoredProjectDefaultProvider(projectId: string): string {
  const getSnapshot = useCallback(() => store.getProjectDefaultSnapshot(projectId), [projectId])
  return useSyncExternalStore(store.subscribe, getSnapshot)
}
