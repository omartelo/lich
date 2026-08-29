import { describe, expect, it, vi } from "vitest"
import type { DetectedProvider } from "./api-types"
import {
  binKey,
  binOffKey,
  climbsToRiskier,
  createProvidersStore,
  enabledKey,
  enabledProviders,
  footerReadout,
  footerReadoutPair,
  readEnabled,
  noProviderInstalled,
  resolveDefaultProvider,
  resolveImplicitSessionKind,
  resolveProjectDefaultProvider,
  sandboxDefaultFor,
  sandboxKey,
  sandboxLevel,
  skipLevel,
  skipLevelPair,
  skipPermissionFlags,
  skipPermissionsKey,
  SANDBOX_RISK_ORDER,
  SKIP_RISK_ORDER,
  type ProviderState,
  type SandboxLevel,
} from "./providers-store"

describe("provider setting keys", () => {
  it("namespaces the enabled flag per provider", () => {
    expect(enabledKey("codex")).toBe("provider.codex.enabled")
  })

  it("keeps the legacy claude.bin key, namespaces the rest", () => {
    expect(binKey("claude")).toBe("claude.bin")
    expect(binKey("codex")).toBe("provider.codex.bin")
    expect(binKey("opencode")).toBe("provider.opencode.bin")
  })

  // The switch that parks a layer hangs off the path's own key, legacy one
  // included — the Go spawn reads these exact strings.
  it("suffixes the parked flag onto the path key", () => {
    expect(binOffKey("claude")).toBe("claude.bin.off")
    expect(binOffKey("codex")).toBe("provider.codex.bin.off")
  })

  // The two scopes are separate rows in the settings table, and the Go spawn
  // reads these exact strings — a rename on either side silently re-arms the
  // wrong checkout, or none.
  it("keys the skip-permissions flag per provider and checkout scope", () => {
    expect(skipPermissionsKey("claude", false)).toBe("provider.claude.skip-permissions")
    expect(skipPermissionsKey("claude", true)).toBe("provider.claude.skip-permissions.worktree")
  })

  // Pinned literals, not a read-back of the map: this string is printed to the
  // user beside the switch that hands an agent the machine, and it has to be
  // the flag the Go spawn actually passes that provider. A provider with no
  // confirmed spelling stays absent — that is what hides the switch.
  it("spells the skip-permissions flag per provider, and only where one is wired", () => {
    expect(skipPermissionFlags.claude).toBe("--dangerously-skip-permissions")
    expect(skipPermissionFlags.codex).toBe("--dangerously-bypass-approvals-and-sandbox")
    expect(skipPermissionFlags.antigravity).toBe("--dangerously-skip-permissions")
    expect(skipPermissionFlags.opencode).toBe("--auto")
    expect(skipPermissionFlags.omp).toBe("--auto-approve")
    expect(skipPermissionFlags.crush).toBe("--yolo")
    // --yolo is Cursor's own alias for it; lich passes the canonical spelling,
    // which is not the one Crush answers to above.
    expect(skipPermissionFlags.cursor).toBe("--force")
    expect(skipPermissionFlags.shell).toBeUndefined()
  })
})

describe("the skip-permissions ladder", () => {
  it("folds the stored pair into a rung", () => {
    expect(skipLevel(false, false)).toBe("never")
    expect(skipLevel(false, true)).toBe("worktrees")
    expect(skipLevel(true, true)).toBe("everywhere")
  })

  // The pair nothing offers any more: free rein in the tree you work in while
  // the throwaway checkout still asks. It has to read as the rung it already
  // behaved like in the project directory, not as the safer one above it.
  it("reads the pair no rung writes as everywhere", () => {
    expect(skipLevel(true, false)).toBe("everywhere")
  })

  it("writes both keys for the chosen rung", () => {
    expect(skipLevelPair("never")).toEqual({ here: false, worktrees: false })
    expect(skipLevelPair("worktrees")).toEqual({ here: false, worktrees: true })
    expect(skipLevelPair("everywhere")).toEqual({ here: true, worktrees: true })
  })

  // A round trip through the control must not move a setting the user did not
  // touch — opening the section and choosing what is already chosen is a no-op.
  it("round-trips every rung", () => {
    for (const level of ["never", "worktrees", "everywhere"] as const) {
      const pair = skipLevelPair(level)
      expect(skipLevel(pair.here, pair.worktrees)).toBe(level)
    }
  })
})

describe("the confirmation gate on a standing safety setting", () => {
  it("asks on the way up the skip ladder", () => {
    expect(climbsToRiskier(SKIP_RISK_ORDER, "never", "worktrees")).toBe(true)
    expect(climbsToRiskier(SKIP_RISK_ORDER, "never", "everywhere")).toBe(true)
    expect(climbsToRiskier(SKIP_RISK_ORDER, "worktrees", "everywhere")).toBe(true)
  })

  // The whole point of the gate: taking the automation away is never the harder
  // direction, so every step down writes straight through.
  it("never asks on the way down the skip ladder", () => {
    expect(climbsToRiskier(SKIP_RISK_ORDER, "everywhere", "worktrees")).toBe(false)
    expect(climbsToRiskier(SKIP_RISK_ORDER, "everywhere", "never")).toBe(false)
    expect(climbsToRiskier(SKIP_RISK_ORDER, "worktrees", "never")).toBe(false)
  })

  // Less confinement is more exposure, so the sandbox ladder is read backwards
  // from the order it is drawn in: "off" is its riskiest rung, not its first.
  it("reads the sandbox ladder safest first", () => {
    expect([...SANDBOX_RISK_ORDER]).toEqual(["everywhere", "worktrees", "ask", "off"])
    expect(climbsToRiskier(SANDBOX_RISK_ORDER, "everywhere", "off")).toBe(true)
    expect(climbsToRiskier(SANDBOX_RISK_ORDER, "worktrees", "ask")).toBe(true)
    expect(climbsToRiskier(SANDBOX_RISK_ORDER, "off", "everywhere")).toBe(false)
    expect(climbsToRiskier(SANDBOX_RISK_ORDER, "ask", "worktrees")).toBe(false)
  })

  // Pressing the rung already chosen is not a move, and a dialog about a write
  // that changes nothing is the cost the gate exists to avoid paying twice.
  it("does not ask for the rung already chosen", () => {
    for (const level of SKIP_RISK_ORDER) {
      expect(climbsToRiskier(SKIP_RISK_ORDER, level, level)).toBe(false)
    }
    for (const level of SANDBOX_RISK_ORDER) {
      expect(climbsToRiskier(SANDBOX_RISK_ORDER, level, level)).toBe(false)
    }
  })
})

describe("the footer readout ladder", () => {
  it("folds the stored pair into a rung", () => {
    expect(footerReadout(false, false)).toBe("off")
    expect(footerReadout(true, false)).toBe("context")
    expect(footerReadout(true, true)).toBe("cost")
  })

  // The pair nothing offers any more: the cost on its own, with no model and no
  // ring beside it. The cost was always the addition, so it reads as the rung
  // that carries it.
  it("reads the pair no rung writes as the cost rung", () => {
    expect(footerReadout(false, true)).toBe("cost")
  })

  it("writes both settings for the chosen rung", () => {
    expect(footerReadoutPair("off")).toEqual({ context: false, cost: false })
    expect(footerReadoutPair("context")).toEqual({ context: true, cost: false })
    expect(footerReadoutPair("cost")).toEqual({ context: true, cost: true })
  })

  it("round-trips every rung", () => {
    for (const level of ["off", "context", "cost"] as const) {
      const pair = footerReadoutPair(level)
      expect(footerReadout(pair.context, pair.cost)).toBe(level)
    }
  })
})

describe("readEnabled", () => {
  it("honors an explicit flag over the default", () => {
    expect(readEnabled("claude", "0")).toBe(false)
    expect(readEnabled("codex", "1")).toBe(true)
  })

  it("defaults claude on and every other provider off when unset", () => {
    expect(readEnabled("claude", "")).toBe(true)
    expect(readEnabled("codex", "")).toBe(false)
    expect(readEnabled("crush", "")).toBe(false)
  })
})

describe("enabledProviders", () => {
  const p = (id: string, enabled: boolean, installed: boolean): ProviderState => ({
    id: id as ProviderState["id"],
    name: id,
    binary: id,
    installed,
    enabled,
    docs: `https://example.test/${id}`,
  })

  it("keeps enabled providers regardless of install state", () => {
    // A claude with a custom bin path (installed=false) must still be offered.
    const list = [p("claude", true, false), p("codex", false, true), p("crush", true, true)]
    expect(enabledProviders(list).map((x) => x.id)).toEqual(["claude", "crush"])
  })
})

describe("resolveDefaultProvider", () => {
  const p = (id: string, enabled: boolean): ProviderState => ({
    id: id as ProviderState["id"],
    name: id,
    binary: id,
    installed: true,
    enabled,
    docs: `https://example.test/${id}`,
  })

  it("uses the stored default when it names an enabled provider", () => {
    const list = [p("claude", true), p("codex", true)]
    expect(resolveDefaultProvider(list, "codex")).toBe("codex")
  })

  it("falls back to the first enabled provider when the default is disabled", () => {
    const list = [p("claude", true), p("codex", false)]
    expect(resolveDefaultProvider(list, "codex")).toBe("claude")
  })

  it("falls back to claude when nothing is enabled or loaded", () => {
    expect(resolveDefaultProvider([], "")).toBe("claude")
    expect(resolveDefaultProvider([p("codex", false)], "")).toBe("claude")
  })
})

describe("resolveProjectDefaultProvider", () => {
  const p = (id: string, enabled: boolean): ProviderState => ({
    id: id as ProviderState["id"],
    name: id,
    binary: id,
    installed: true,
    enabled,
    docs: `https://example.test/${id}`,
  })

  it("prefers an enabled project override to the global default", () => {
    const list = [p("claude", true), p("codex", true)]
    expect(resolveProjectDefaultProvider(list, "claude", "codex")).toBe("codex")
  })

  it("inherits the global default when the project value is empty", () => {
    const list = [p("claude", true), p("codex", true)]
    expect(resolveProjectDefaultProvider(list, "codex", "")).toBe("codex")
  })

  it("ignores a disabled project override and resolves the global fallback", () => {
    const list = [p("claude", true), p("codex", false)]
    expect(resolveProjectDefaultProvider(list, "claude", "codex")).toBe("claude")
  })
})

// The zero-agent machine: every fallback below still lands on Claude, so the
// gate in front of them is what stops an implicit session opening on
// `claude: command not found`.
describe("noProviderInstalled / resolveImplicitSessionKind", () => {
  const p = (id: string, installed: boolean, enabled: boolean): ProviderState => ({
    id: id as ProviderState["id"],
    name: id,
    binary: id,
    installed,
    enabled,
    docs: `https://example.test/${id}`,
  })

  it("reads an empty list as detection not having answered, never as a bare machine", () => {
    expect(noProviderInstalled([])).toBe(false)
    expect(resolveImplicitSessionKind([], "", "")).toBe("claude")
  })

  it("spots the machine with nothing on PATH", () => {
    expect(noProviderInstalled([p("claude", false, true), p("codex", false, false)])).toBe(true)
    expect(noProviderInstalled([p("claude", false, true), p("codex", true, false)])).toBe(false)
  })

  // Claude is enabled by default (readEnabled) even with no binary anywhere, so
  // this is the exact list a first run on a bare machine produces.
  it("spawns a shell when no provider is installed, whatever the stored default says", () => {
    const bare = [p("claude", false, true), p("codex", false, false)]
    expect(resolveImplicitSessionKind(bare, "", "")).toBe("shell")
    expect(resolveImplicitSessionKind(bare, "claude", "codex")).toBe("shell")
  })

  it("defers to the project resolution the moment one provider is installed", () => {
    const list = [p("claude", true, true), p("codex", true, true)]
    expect(resolveImplicitSessionKind(list, "claude", "codex")).toBe("codex")
    expect(resolveImplicitSessionKind(list, "codex", "")).toBe("codex")
  })

  // A custom binary path leaves installed=false on every row, so this machine
  // gets a terminal it did not need. Deliberate, and in docs/ceilings.md.
  it("still resolves the enabled provider once anything else is on PATH", () => {
    const list = [p("claude", false, true), p("crush", true, false)]
    expect(resolveImplicitSessionKind(list, "claude", "")).toBe("claude")
  })
})

describe("createProvidersStore", () => {
  const detected: DetectedProvider[] = [
    {
      id: "claude",
      name: "Claude Code",
      binary: "claude",
      installed: true,
      path: "/usr/bin/claude",
      docs: "https://code.claude.com/docs/en/setup",
    },
    // A binary that is not the id, which is what antigravity ships as.
    {
      id: "codex",
      name: "Codex",
      binary: "cdx",
      installed: false,
      path: "",
      docs: "https://d/codex",
    },
    // unknown id
    { id: "mystery", name: "Mystery", binary: "mystery", installed: true, path: "/x", docs: "" },
  ]

  function build(enabledValues: Record<string, string> = {}, defaultValue = "") {
    const persistEnabled = vi.fn()
    const persistDefault = vi.fn()
    const persistProjectDefault = vi.fn()
    const store = createProvidersStore({
      detect: async () => detected,
      getEnabled: async (id) => enabledValues[id] ?? "",
      persistEnabled,
      getDefault: async () => defaultValue,
      persistDefault,
      persistProjectDefault,
    })
    return { store, persistEnabled, persistDefault, persistProjectDefault }
  }

  it("loads only known providers with their install + default enabled state", async () => {
    const { store } = build()
    await store.load()
    const got = store.getSnapshot()
    // "mystery" is dropped; claude defaults enabled, codex opt-in default off.
    expect(got.map((p) => p.id)).toEqual(["claude", "codex"])
    expect(got.find((p) => p.id === "claude")?.enabled).toBe(true)
    expect(got.find((p) => p.id === "codex")?.enabled).toBe(false)
    expect(got.find((p) => p.id === "codex")?.installed).toBe(false)
    // Carried through, because the settings screen asks $PATH for this and not
    // for the id.
    expect(got.find((p) => p.id === "codex")?.binary).toBe("cdx")
  })

  it("honors a stored enabled flag over the default", async () => {
    const { store } = build({ claude: "0", codex: "1" })
    await store.load()
    const got = store.getSnapshot()
    expect(got.find((p) => p.id === "claude")?.enabled).toBe(false)
    expect(got.find((p) => p.id === "codex")?.enabled).toBe(true)
  })

  it("setEnabled updates the snapshot, notifies, and persists", async () => {
    const { store, persistEnabled } = build()
    await store.load()
    const seen = vi.fn()
    const off = store.subscribe(seen)
    store.setEnabled("codex", true)
    expect(store.getSnapshot().find((p) => p.id === "codex")?.enabled).toBe(true)
    expect(seen).toHaveBeenCalledTimes(1)
    expect(persistEnabled).toHaveBeenCalledWith("codex", "1")
    off()
    store.setEnabled("codex", false)
    expect(seen).toHaveBeenCalledTimes(1) // unsubscribed
    expect(persistEnabled).toHaveBeenLastCalledWith("codex", "0")
  })

  it("ensureLoaded loads once", async () => {
    const detect = vi.fn(async () => detected)
    const store = createProvidersStore({
      detect,
      getEnabled: async () => "",
      persistEnabled: vi.fn(),
      getDefault: async () => "",
      persistDefault: vi.fn(),
      persistProjectDefault: vi.fn(),
    })
    store.ensureLoaded()
    store.ensureLoaded()
    await vi.waitFor(() => expect(store.getSnapshot().length).toBe(2))
    expect(detect).toHaveBeenCalledTimes(1)
  })

  it("carries each provider's install page onto the snapshot", async () => {
    const { store } = build()
    await store.load()
    expect(store.getSnapshot().find((p) => p.id === "claude")?.docs).toBe(
      "https://code.claude.com/docs/en/setup",
    )
  })

  // The whole point of the button: a provider installed while lich is open shows
  // up without a relaunch.
  it("refresh re-probes PATH and notifies", async () => {
    let probe: DetectedProvider[] = [
      {
        id: "claude",
        name: "Claude Code",
        binary: "claude",
        installed: false,
        path: "",
        docs: "d",
      },
    ]
    const store = createProvidersStore({
      detect: async () => probe,
      getEnabled: async () => "",
      persistEnabled: vi.fn(),
      getDefault: async () => "",
      persistDefault: vi.fn(),
      persistProjectDefault: vi.fn(),
    })
    await store.load()
    expect(store.getSnapshot()[0].installed).toBe(false)

    const seen = vi.fn()
    store.subscribe(seen)
    probe = [
      {
        id: "claude",
        name: "Claude Code",
        binary: "claude",
        installed: true,
        path: "/usr/bin/claude",
        docs: "d",
      },
    ]
    await store.refresh()
    expect(store.getSnapshot()[0].installed).toBe(true)
    expect(seen).toHaveBeenCalledTimes(1)
  })

  // A toggle persists through a fire-and-forget write, so re-reading the settings
  // on every refresh would let a slow write flip the switch back under the user.
  it("refresh keeps the enabled flags and the default it already has", async () => {
    const getEnabled = vi.fn(async () => "")
    const store = createProvidersStore({
      detect: async () => detected,
      getEnabled,
      persistEnabled: vi.fn(),
      getDefault: async () => "codex",
      persistDefault: vi.fn(),
      persistProjectDefault: vi.fn(),
    })
    await store.load()
    store.setEnabled("codex", true)
    const reads = getEnabled.mock.calls.length

    await store.refresh()
    expect(store.getSnapshot().find((p) => p.id === "codex")?.enabled).toBe(true)
    expect(store.getDefaultSnapshot()).toBe("codex")
    expect(getEnabled).toHaveBeenCalledTimes(reads)
  })

  it("refresh drops a provider id this build does not know", async () => {
    const { store } = build()
    await store.refresh()
    expect(store.getSnapshot().map((p) => p.id)).toEqual(["claude", "codex"])
  })

  it("getProjectSessionKind falls to the shell only while nothing is installed", async () => {
    const { store } = build({ claude: "1" })
    await store.load()
    // codex is the only not-installed row; claude is on PATH, so a project with
    // no override gets the provider.
    expect(store.getProjectSessionKind("proj")).toBe("claude")
  })

  it("loads the stored default provider", async () => {
    const { store } = build({ codex: "1" }, "codex")
    await store.load()
    expect(store.getDefaultSnapshot()).toBe("codex")
  })

  it("setDefault updates the snapshot, notifies, and persists", async () => {
    const { store, persistDefault } = build()
    await store.load()
    const seen = vi.fn()
    const off = store.subscribe(seen)
    store.setDefault("codex")
    expect(store.getDefaultSnapshot()).toBe("codex")
    expect(seen).toHaveBeenCalledTimes(1)
    expect(persistDefault).toHaveBeenCalledWith("codex")
    off()
    store.setDefault("claude")
    expect(seen).toHaveBeenCalledTimes(1) // unsubscribed
    expect(persistDefault).toHaveBeenLastCalledWith("claude")
  })

  it("hydrates project overrides synchronously and notifies subscribers", () => {
    const { store } = build()
    const seen = vi.fn()
    store.subscribe(seen)

    store.hydrateProjectDefaults([
      { id: "p1", defaultProvider: "codex" },
      { id: "p2", defaultProvider: "" },
    ])

    expect(store.getProjectDefaultSnapshot("p1")).toBe("codex")
    expect(store.getProjectDefaultSnapshot("p2")).toBe("")
    expect(seen).toHaveBeenCalledTimes(1)
  })

  it("retries a failed provider load", async () => {
    const detect = vi
      .fn<() => Promise<DetectedProvider[]>>()
      .mockRejectedValueOnce(new Error("detection failed"))
      .mockResolvedValue(detected)
    const store = createProvidersStore({
      detect,
      getEnabled: async () => "",
      persistEnabled: vi.fn(),
      getDefault: async () => "claude",
      persistDefault: vi.fn(),
      persistProjectDefault: vi.fn(),
    })

    await expect(store.load()).rejects.toThrow("detection failed")
    expect(store.getSnapshot()).toEqual([])
    await expect(store.load()).resolves.toBeUndefined()
    expect(store.getSnapshot().map((provider) => provider.id)).toEqual(["claude", "codex"])
    expect(detect).toHaveBeenCalledTimes(2)
  })

  it("shares one in-flight load between concurrent callers", async () => {
    const detect = vi.fn<() => Promise<DetectedProvider[]>>().mockResolvedValue(detected)
    const store = createProvidersStore({
      detect,
      getEnabled: async () => "",
      persistEnabled: vi.fn(),
      getDefault: async () => "claude",
      persistDefault: vi.fn(),
      persistProjectDefault: vi.fn(),
    })

    await Promise.all([store.load(), store.load(), store.load()])
    expect(detect).toHaveBeenCalledTimes(1)

    // And a later caller finds it ready rather than detecting again.
    await store.load()
    expect(detect).toHaveBeenCalledTimes(1)
  })

  it("sets and clears a project override synchronously", () => {
    const { store, persistProjectDefault } = build()
    const seen = vi.fn()
    store.subscribe(seen)

    store.setProjectDefault("p1", "codex")
    expect(store.getProjectDefaultSnapshot("p1")).toBe("codex")
    expect(persistProjectDefault).toHaveBeenLastCalledWith("p1", "codex")

    store.setProjectDefault("p1", "")
    expect(store.getProjectDefaultSnapshot("p1")).toBe("")
    expect(persistProjectDefault).toHaveBeenLastCalledWith("p1", "")
    expect(seen).toHaveBeenCalledTimes(2)
  })

  it("keeps a cleared project override inherited across later global changes", async () => {
    const { store } = build({ codex: "1", opencode: "1" }, "claude")
    await store.load()
    store.setProjectDefault("p1", "codex")
    store.setProjectDefault("p1", "")

    expect(store.getProjectProviderKind("p1")).toBe("claude")

    store.setDefault("codex")
    expect(store.getProjectProviderKind("p1")).toBe("codex")
  })

  it("falls back to the global default when a hydrated override is disabled", async () => {
    const { store } = build({ claude: "1", codex: "0" }, "claude")
    store.hydrateProjectDefaults([{ id: "p1", defaultProvider: "codex" }])
    await store.load()

    expect(store.getProjectProviderKind("p1")).toBe("claude")
  })
})

describe("sandbox rung", () => {
  it("keys the setting per provider, mirroring the Go spelling", () => {
    expect(sandboxKey("claude")).toBe("provider.claude.sandbox")
    expect(sandboxKey("crush")).toBe("provider.crush.sandbox")
  })

  it("reads every rung the ladder names", () => {
    for (const level of ["off", "ask", "worktrees", "everywhere"] as const) {
      expect(sandboxLevel(level)).toBe(level)
    }
  })

  // An unknown value must never confine a session nobody asked to confine, and
  // "off" is the only answer that surprises no one — it is what lich did before.
  it("reads anything else as off", () => {
    for (const value of ["", "true", "on", "Everywhere", " worktrees", "always"]) {
      expect(sandboxLevel(value)).toBe("off")
    }
  })

  // The dialog has to arrive showing the answer the spawn would reach on its
  // own, or the box the user leaves untouched means something else.
  it("matches store.SandboxDefault on both sides of the checkout", () => {
    const cases: [SandboxLevel, boolean, boolean][] = [
      ["off", false, false],
      ["off", true, false],
      ["ask", false, false],
      ["ask", true, false],
      ["worktrees", false, false],
      ["worktrees", true, true],
      ["everywhere", false, true],
      ["everywhere", true, true],
    ]
    for (const [level, worktree, want] of cases) {
      expect(sandboxDefaultFor(level, worktree)).toBe(want)
    }
  })
})
