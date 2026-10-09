// @vitest-environment jsdom
//
// index.html paints the persisted theme before React starts, from the keys the
// settings provider writes. This runs that inline script itself against those
// keys, so a renamed key or a changed snapshot shape fails here instead of
// coming back as a white flash on launch.
import { afterEach, describe, expect, it, vi } from "vitest"
import type { ThemeDefinition } from "@/lib/api-types"
import {
  BUNDLED_THEMES,
  THEME_BOOT_STORAGE_KEY,
  THEME_STORAGE_KEY,
  themeBootSnapshot,
} from "@/lib/themes"
import html from "../../index.html?raw"

const script = html.match(/<script id="theme-boot">([\s\S]*?)<\/script>/)?.[1]

const root = document.documentElement

interface Launch {
  selected?: string
  snapshot?: string
  systemDark: boolean
}

function launch({ selected, snapshot, systemDark }: Launch) {
  if (selected !== undefined) localStorage.setItem(THEME_STORAGE_KEY, selected)
  if (snapshot !== undefined) localStorage.setItem(THEME_BOOT_STORAGE_KEY, snapshot)
  vi.stubGlobal("matchMedia", (query: string) => ({
    matches: systemDark && query === "(prefers-color-scheme: dark)",
  }))
  if (script === undefined) throw new Error("index.html has no theme-boot script")
  new Function(script)()
}

const custom: ThemeDefinition = {
  ...BUNDLED_THEMES[1],
  id: "midnight",
  name: "Midnight",
  origin: "custom",
  app: { ...BUNDLED_THEMES[1].app, background: "oklch(0.1 0.02 260)" },
}

afterEach(() => {
  localStorage.clear()
  root.className = ""
  root.removeAttribute("style")
  vi.unstubAllGlobals()
})

describe("theme boot", () => {
  it("paints the bundled dark theme against a light system", () => {
    launch({ selected: "dark", systemDark: false })
    expect(root.classList.contains("dark")).toBe(true)
  })

  it("paints the bundled light theme against a dark system", () => {
    launch({ selected: "light", systemDark: true })
    expect(root.classList.contains("dark")).toBe(false)
  })

  it("follows the system when nothing was ever chosen", () => {
    launch({ systemDark: true })
    expect(root.classList.contains("dark")).toBe(true)
  })

  it("asks the system again for System, whatever the last launch painted", () => {
    const snapshot = themeBootSnapshot("system", BUNDLED_THEMES[1])
    launch({ selected: "system", snapshot, systemDark: false })
    expect(root.classList.contains("dark")).toBe(false)
  })

  // A custom theme's colors only arrive over RPC, after first paint: the
  // snapshot the provider left behind is what the window opens in.
  it("paints a custom theme from the snapshot of its last launch", () => {
    const snapshot = themeBootSnapshot("midnight", custom)
    launch({ selected: "midnight", snapshot, systemDark: false })
    expect(root.classList.contains("dark")).toBe(true)
    expect(root.style.getPropertyValue("--background")).toBe("oklch(0.1 0.02 260)")
  })

  it("ignores a snapshot taken for another selection", () => {
    const snapshot = themeBootSnapshot("midnight", custom)
    launch({ selected: "light", snapshot, systemDark: true })
    expect(root.classList.contains("dark")).toBe(false)
    expect(root.style.getPropertyValue("--background")).toBe("")
  })
})
