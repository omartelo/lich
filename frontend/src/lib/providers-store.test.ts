import { describe, expect, it, vi } from "vitest"
import type { DetectedProvider } from "./api-types"
import {
  binKey,
  createProvidersStore,
  enabledKey,
  enabledProviders,
  readEnabled,
  type ProviderState,
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
    installed,
    enabled,
  })

  it("keeps enabled providers regardless of install state", () => {
    // A claude with a custom bin path (installed=false) must still be offered.
    const list = [p("claude", true, false), p("codex", false, true), p("crush", true, true)]
    expect(enabledProviders(list).map((x) => x.id)).toEqual(["claude", "crush"])
  })
})

describe("createProvidersStore", () => {
  const detected: DetectedProvider[] = [
    { id: "claude", name: "Claude Code", installed: true, path: "/usr/bin/claude" },
    { id: "codex", name: "Codex", installed: false, path: "" },
    { id: "mystery", name: "Mystery", installed: true, path: "/x" }, // unknown id
  ]

  function build(enabledValues: Record<string, string> = {}) {
    const persistEnabled = vi.fn()
    const store = createProvidersStore({
      detect: async () => detected,
      getEnabled: async (id) => enabledValues[id] ?? "",
      persistEnabled,
    })
    return { store, persistEnabled }
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
    })
    store.ensureLoaded()
    store.ensureLoaded()
    await vi.waitFor(() => expect(store.getSnapshot().length).toBe(2))
    expect(detect).toHaveBeenCalledTimes(1)
  })
})
