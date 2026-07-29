import { createContext, useCallback, useContext, useEffect, useState } from "react"
import type { ReactNode } from "react"
import {
  DEFAULT_HOTKEYS,
  isRecordingTarget,
  loadHotkeys,
  saveHotkeys,
  type Combo,
  type HotkeyId,
  type Hotkeys,
} from "@/lib/hotkeys"
import { zoomIntent } from "@/lib/terminal/zoom-keys"
import { parseBoolPref, parseNumberPref, readPref, writePref } from "@/lib/prefs"
import { Themes as ThemeRPC } from "@/lib/rpc"
import {
  applyAppTheme,
  BUNDLED_THEMES,
  DEFAULT_TERMINAL_THEME,
  DEFAULT_THEME,
  mergeThemes,
  resolveTerminalTheme,
  resolveTheme,
  sanitizeTerminalThemePreference,
  sanitizeThemePreference,
} from "@/lib/themes"
import type { TerminalTheme, Theme, ResolvedTheme } from "@/lib/themes"
import type { ThemeDefinition } from "@/lib/api-types"

export type { TerminalTheme, Theme, ResolvedTheme } from "@/lib/themes"
export { DEFAULT_TERMINAL_THEME, DEFAULT_THEME } from "@/lib/themes"

const FONT_STORAGE_KEY = "lich.terminal.font"
const TERMINAL_FONT_SIZE_STORAGE_KEY = "lich.terminal.fontSize"
const THEME_STORAGE_KEY = "lich.appearance.theme"
const ZOOM_STORAGE_KEY = "lich.appearance.zoom"
const TERMINAL_THEME_STORAGE_KEY = "lich.appearance.terminalTheme"
const CONTEXT_USAGE_STORAGE_KEY = "lich.footer.contextUsage"

// DEFAULT_FONT is the bundled FiraCode Nerd Font Mono. It is not installed via
// fontconfig, so it must be offered explicitly alongside the system fonts.
export const DEFAULT_FONT = "FiraCode Nerd Font Mono"

export const ZOOM_MIN = 0.5
export const ZOOM_MAX = 2
export const ZOOM_STEP = 0.1
export const DEFAULT_ZOOM = 1

// clampZoom bounds a zoom factor and snaps it to one decimal so repeated
// step arithmetic does not drift (0.1 + 0.2 ...).
export function clampZoom(value: number): number {
  const bounded = Math.min(ZOOM_MAX, Math.max(ZOOM_MIN, value))
  return Math.round(bounded * 10) / 10
}

// Terminal text size, in absolute px. It is deliberately not part of the app
// zoom: zoom rescales the chrome in rem, and the terminal stays out of it so a
// zoom step never changes the PTY's cols×rows and never rewraps a running TUI.
// Changing this one does resize the grid — that is the point of it.
export const TERMINAL_FONT_SIZE_MIN = 8
export const TERMINAL_FONT_SIZE_MAX = 32
export const TERMINAL_FONT_SIZE_STEP = 1
export const DEFAULT_TERMINAL_FONT_SIZE = 14

export function clampTerminalFontSize(value: number): number {
  const bounded = Math.min(TERMINAL_FONT_SIZE_MAX, Math.max(TERMINAL_FONT_SIZE_MIN, value))
  return Math.round(bounded)
}

const readFont = (): string => readPref(FONT_STORAGE_KEY) ?? DEFAULT_FONT

const readTerminalFontSize = (): number =>
  parseNumberPref(
    readPref(TERMINAL_FONT_SIZE_STORAGE_KEY),
    DEFAULT_TERMINAL_FONT_SIZE,
    clampTerminalFontSize,
  )

const readTheme = (): Theme => sanitizeThemePreference(readPref(THEME_STORAGE_KEY))

const readTerminalTheme = (): TerminalTheme =>
  sanitizeTerminalThemePreference(readPref(TERMINAL_THEME_STORAGE_KEY))

const readZoom = (): number => parseNumberPref(readPref(ZOOM_STORAGE_KEY), DEFAULT_ZOOM, clampZoom)

// Default on: the footer context readout shows unless the user turned it off.
const readContextUsage = (): boolean => parseBoolPref(readPref(CONTEXT_USAGE_STORAGE_KEY), true)

interface SettingsValue {
  /** Terminal font family, applied globally across all project terminals. */
  font: string
  setFont: (font: string) => void
  /** Terminal text size in px. Independent of `zoom` — see the constants. */
  terminalFontSize: number
  setTerminalFontSize: (size: number) => void
  /** Color theme applied to the whole app via the `.dark` class on <html>. */
  theme: Theme
  setTheme: (theme: Theme) => void
  /** Bundled and imported themes available to the app. */
  themes: ThemeDefinition[]
  importTheme: (raw: string) => Promise<ThemeDefinition>
  removeTheme: (id: string) => Promise<void>
  /** Theme resolved to concrete colors (system already mapped to the OS). */
  resolvedTheme: ResolvedTheme
  /** UI zoom factor applied to the whole app (1 = 100%). */
  zoom: number
  setZoom: (zoom: number) => void
  /** Terminal background theme selection. */
  terminalTheme: TerminalTheme
  setTerminalTheme: (theme: TerminalTheme) => void
  /** Terminal theme resolved to a concrete scheme (match already mapped). */
  resolvedTerminalTheme: ResolvedTheme
  /** Configurable global keyboard shortcuts, keyed by action. */
  hotkeys: Hotkeys
  setHotkey: (id: HotkeyId, combo: Combo) => void
  resetHotkey: (id: HotkeyId) => void
  /** Whether the footer shows the active session's context-window usage. */
  showContextUsage: boolean
  setShowContextUsage: (show: boolean) => void
}

const SettingsContext = createContext<SettingsValue | null>(null)

export function SettingsProvider({ children }: { children: ReactNode }) {
  const [font, setFontState] = useState<string>(readFont)
  const [terminalFontSize, setTerminalFontSizeState] = useState<number>(readTerminalFontSize)
  const [themes, setThemes] = useState<ThemeDefinition[]>(BUNDLED_THEMES)
  const [theme, setThemeState] = useState<Theme>(readTheme)
  const [resolvedTheme, setResolvedTheme] = useState<ResolvedTheme>(BUNDLED_THEMES[0])
  const [zoom, setZoomState] = useState<number>(readZoom)
  const [terminalTheme, setTerminalThemeState] = useState<TerminalTheme>(readTerminalTheme)
  const [hotkeys, setHotkeys] = useState<Hotkeys>(loadHotkeys)
  const [showContextUsage, setShowContextUsageState] = useState<boolean>(readContextUsage)

  useEffect(() => {
    let cancelled = false
    ThemeRPC.List()
      .then((loaded) => {
        if (!cancelled) {
          setThemes(mergeThemes(loaded))
        }
      })
      .catch((error) => {
        console.warn("[settings] failed to load custom themes", error)
      })
    return () => {
      cancelled = true
    }
  }, [])

  const setFont = useCallback((next: string) => {
    setFontState(next)
    writePref(FONT_STORAGE_KEY, next)
  }, [])

  const setTerminalFontSize = useCallback((next: number) => {
    const clamped = clampTerminalFontSize(next)
    setTerminalFontSizeState(clamped)
    writePref(TERMINAL_FONT_SIZE_STORAGE_KEY, clamped)
  }, [])

  const setTheme = useCallback((next: Theme) => {
    setThemeState(next)
    writePref(THEME_STORAGE_KEY, next)
  }, [])

  const setZoom = useCallback((next: number) => {
    const clamped = clampZoom(next)
    setZoomState(clamped)
    writePref(ZOOM_STORAGE_KEY, clamped)
  }, [])

  // zoomBy applies a relative step off the latest value so rapid wheel ticks
  // accumulate instead of collapsing to a single step between renders.
  const zoomBy = useCallback((delta: number) => {
    setZoomState((prev) => {
      const clamped = clampZoom(prev + delta)
      writePref(ZOOM_STORAGE_KEY, clamped)
      return clamped
    })
  }, [])

  const setTerminalTheme = useCallback((next: TerminalTheme) => {
    setTerminalThemeState(next)
    writePref(TERMINAL_THEME_STORAGE_KEY, next)
  }, [])

  const importTheme = useCallback(
    async (raw: string) => {
      const imported = await ThemeRPC.Import(raw)
      setThemes((prev) => mergeThemes([...prev, imported]))
      setTheme(imported.id)
      return imported
    },
    [setTheme],
  )

  const removeTheme = useCallback(async (id: string) => {
    await ThemeRPC.Remove(id)
    setThemes((prev) => mergeThemes(prev.filter((item) => item.id !== id)))
    setThemeState((prev) => {
      if (prev !== id) {
        return prev
      }
      writePref(THEME_STORAGE_KEY, DEFAULT_THEME)
      return DEFAULT_THEME
    })
    setTerminalThemeState((prev) => {
      if (prev !== id) {
        return prev
      }
      writePref(TERMINAL_THEME_STORAGE_KEY, DEFAULT_TERMINAL_THEME)
      return DEFAULT_TERMINAL_THEME
    })
  }, [])

  const setHotkey = useCallback((id: HotkeyId, combo: Combo) => {
    setHotkeys((prev) => {
      const next = { ...prev, [id]: combo }
      saveHotkeys(next)
      return next
    })
  }, [])

  const resetHotkey = useCallback((id: HotkeyId) => {
    setHotkeys((prev) => {
      const next = { ...prev, [id]: DEFAULT_HOTKEYS[id] }
      saveHotkeys(next)
      return next
    })
  }, [])

  const setShowContextUsage = useCallback((next: boolean) => {
    setShowContextUsageState(next)
    writePref(CONTEXT_USAGE_STORAGE_KEY, next)
  }, [])

  // Apply the resolved theme's CSS variables and toggle `.dark` for existing
  // dark variants. For "system", follow the OS scheme and keep following it
  // live.
  useEffect(() => {
    const media = window.matchMedia("(prefers-color-scheme: dark)")
    const apply = () => {
      const resolved = resolveTheme(theme, themes, media.matches)
      applyAppTheme(resolved, document.documentElement)
      document.documentElement.classList.toggle("dark", resolved.scheme === "dark")
      setResolvedTheme(resolved)
    }
    apply()
    if (theme !== "system") return
    media.addEventListener("change", apply)
    return () => media.removeEventListener("change", apply)
  }, [theme, themes])

  // Scale the app by moving the root font size: every Tailwind spacing and type
  // utility resolves in rem (--spacing is 0.25rem, --text-* are rem), so one
  // percentage rescales the whole chrome and reflows it honestly.
  //
  // A percentage, not a px value, so a user who raised Chromium's default font
  // size keeps that as their 100%.
  //
  // This deliberately replaces CSS `zoom` on the root, which had two flaws:
  // `zoom` does not scale vh/vw, so the 100vh/100vw root rendered at viewport ×
  // zoom (window left showing through when zoomed out, layout cut by the page's
  // overflow:hidden when zoomed in); and it scaled the terminal along with
  // everything else, which — once the root filled the window — handed the
  // terminal more CSS pixels and reflowed the PTY to a different cols×rows on
  // every zoom step, wrapping whatever TUI was running. The terminal sizes its
  // text in absolute px (TerminalView's fontSize), so rem scaling leaves it
  // untouched by construction; its size is its own setting.
  useEffect(() => {
    document.documentElement.style.fontSize = `${zoom * 100}%`
  }, [zoom])

  // Zoom via keyboard chords or Ctrl/Cmd + mouse wheel. Both listen on the
  // capture phase so they win even inside a terminal, which otherwise swallows
  // modifier chords and wheel events; propagation is stopped so the PTY never
  // sees them. Both also preventDefault, which is what keeps Chromium's own
  // zoom accelerator from running on top of this one — miss that on any single
  // chord and the app and the browser each apply a zoom (see zoom-keys.ts).
  // The wheel listener is non-passive to allow preventDefault, and bails on
  // non-Ctrl scrolls so normal scrolling still works.
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (isRecordingTarget(event)) return
      const intent = zoomIntent(event)
      if (!intent) return
      event.preventDefault()
      event.stopPropagation()
      if (intent === "reset") {
        setZoom(DEFAULT_ZOOM)
        return
      }
      zoomBy(intent === "in" ? ZOOM_STEP : -ZOOM_STEP)
    }
    const onWheel = (event: WheelEvent) => {
      if (!event.ctrlKey) return
      event.preventDefault()
      event.stopPropagation()
      zoomBy(event.deltaY < 0 ? ZOOM_STEP : -ZOOM_STEP)
    }
    window.addEventListener("keydown", onKey, true)
    window.addEventListener("wheel", onWheel, { capture: true, passive: false })
    return () => {
      window.removeEventListener("keydown", onKey, true)
      window.removeEventListener("wheel", onWheel, true)
    }
  }, [zoomBy, setZoom])

  const resolvedTerminalTheme = resolveTerminalTheme(terminalTheme, resolvedTheme, themes)

  return (
    <SettingsContext.Provider
      value={{
        font,
        setFont,
        terminalFontSize,
        setTerminalFontSize,
        themes,
        theme,
        setTheme,
        importTheme,
        removeTheme,
        resolvedTheme,
        zoom,
        setZoom,
        terminalTheme,
        setTerminalTheme,
        resolvedTerminalTheme,
        hotkeys,
        setHotkey,
        resetHotkey,
        showContextUsage,
        setShowContextUsage,
      }}
    >
      {children}
    </SettingsContext.Provider>
  )
}

export function useSettings(): SettingsValue {
  const ctx = useContext(SettingsContext)
  if (!ctx) {
    throw new Error("useSettings must be used within a SettingsProvider")
  }
  return ctx
}
