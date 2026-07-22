import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useState,
} from "react"
import type { ReactNode } from "react"
import {
  DEFAULT_HOTKEYS,
  isRecordingTarget,
  loadHotkeys,
  saveHotkeys,
  type Combo,
  type HotkeyId,
  type Hotkeys,
} from "./hotkeys"
import { zoomIntent } from "./zoom-keys"

const FONT_STORAGE_KEY = "lich.terminal.font"
const TERMINAL_FONT_SIZE_STORAGE_KEY = "lich.terminal.fontSize"
const THEME_STORAGE_KEY = "lich.appearance.theme"
const ZOOM_STORAGE_KEY = "lich.appearance.zoom"
const TERMINAL_THEME_STORAGE_KEY = "lich.appearance.terminalTheme"

// DEFAULT_FONT is the bundled FiraCode Nerd Font Mono. It is not installed via
// fontconfig, so it must be offered explicitly alongside the system fonts.
export const DEFAULT_FONT = "FiraCode Nerd Font Mono"

// THEMES drives both the persisted value and the Appearance picker options.
// "system" follows the OS color scheme live.
export const THEMES = ["system", "light", "dark"] as const
export type Theme = (typeof THEMES)[number]
export const DEFAULT_THEME: Theme = "system"

// TERMINAL_THEMES: "match" tracks the resolved app theme; the others force it.
export const TERMINAL_THEMES = ["match", "light", "dark"] as const
export type TerminalTheme = (typeof TERMINAL_THEMES)[number]
export const DEFAULT_TERMINAL_THEME: TerminalTheme = "match"

// A theme resolved to a concrete color scheme (system/match already applied).
export type ResolvedTheme = "light" | "dark"

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
  const bounded = Math.min(
    TERMINAL_FONT_SIZE_MAX,
    Math.max(TERMINAL_FONT_SIZE_MIN, value),
  )
  return Math.round(bounded)
}

function readTerminalFontSize(): number {
  const stored = Number(localStorage.getItem(TERMINAL_FONT_SIZE_STORAGE_KEY))
  return Number.isFinite(stored) && stored > 0
    ? clampTerminalFontSize(stored)
    : DEFAULT_TERMINAL_FONT_SIZE
}

function readTheme(): Theme {
  const stored = localStorage.getItem(THEME_STORAGE_KEY)
  return THEMES.includes(stored as Theme) ? (stored as Theme) : DEFAULT_THEME
}

function readTerminalTheme(): TerminalTheme {
  const stored = localStorage.getItem(TERMINAL_THEME_STORAGE_KEY)
  return TERMINAL_THEMES.includes(stored as TerminalTheme)
    ? (stored as TerminalTheme)
    : DEFAULT_TERMINAL_THEME
}

function readZoom(): number {
  const stored = Number(localStorage.getItem(ZOOM_STORAGE_KEY))
  return Number.isFinite(stored) && stored > 0 ? clampZoom(stored) : DEFAULT_ZOOM
}

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
  /** Theme resolved to a concrete scheme (system already mapped to the OS). */
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
}

const SettingsContext = createContext<SettingsValue | null>(null)

export function SettingsProvider({ children }: { children: ReactNode }) {
  const [font, setFontState] = useState<string>(
    () => localStorage.getItem(FONT_STORAGE_KEY) ?? DEFAULT_FONT,
  )
  const [terminalFontSize, setTerminalFontSizeState] =
    useState<number>(readTerminalFontSize)
  const [theme, setThemeState] = useState<Theme>(readTheme)
  const [resolvedTheme, setResolvedTheme] = useState<ResolvedTheme>("light")
  const [zoom, setZoomState] = useState<number>(readZoom)
  const [terminalTheme, setTerminalThemeState] =
    useState<TerminalTheme>(readTerminalTheme)
  const [hotkeys, setHotkeys] = useState<Hotkeys>(loadHotkeys)

  const setFont = useCallback((next: string) => {
    setFontState(next)
    localStorage.setItem(FONT_STORAGE_KEY, next)
  }, [])

  const setTerminalFontSize = useCallback((next: number) => {
    const clamped = clampTerminalFontSize(next)
    setTerminalFontSizeState(clamped)
    localStorage.setItem(TERMINAL_FONT_SIZE_STORAGE_KEY, String(clamped))
  }, [])

  const setTheme = useCallback((next: Theme) => {
    setThemeState(next)
    localStorage.setItem(THEME_STORAGE_KEY, next)
  }, [])

  const setZoom = useCallback((next: number) => {
    const clamped = clampZoom(next)
    setZoomState(clamped)
    localStorage.setItem(ZOOM_STORAGE_KEY, String(clamped))
  }, [])

  // zoomBy applies a relative step off the latest value so rapid wheel ticks
  // accumulate instead of collapsing to a single step between renders.
  const zoomBy = useCallback((delta: number) => {
    setZoomState((prev) => {
      const clamped = clampZoom(prev + delta)
      localStorage.setItem(ZOOM_STORAGE_KEY, String(clamped))
      return clamped
    })
  }, [])

  const setTerminalTheme = useCallback((next: TerminalTheme) => {
    setTerminalThemeState(next)
    localStorage.setItem(TERMINAL_THEME_STORAGE_KEY, next)
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

  // Toggle the `.dark` class on <html> and track the resolved scheme. For
  // "system", follow the OS scheme and keep following it live.
  useEffect(() => {
    const media = window.matchMedia("(prefers-color-scheme: dark)")
    const apply = () => {
      const dark = theme === "dark" || (theme === "system" && media.matches)
      document.documentElement.classList.toggle("dark", dark)
      setResolvedTheme(dark ? "dark" : "light")
    }
    apply()
    if (theme !== "system") return
    media.addEventListener("change", apply)
    return () => media.removeEventListener("change", apply)
  }, [theme])

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

  const resolvedTerminalTheme: ResolvedTheme =
    terminalTheme === "match" ? resolvedTheme : terminalTheme

  return (
    <SettingsContext.Provider
      value={{
        font,
        setFont,
        terminalFontSize,
        setTerminalFontSize,
        theme,
        setTheme,
        resolvedTheme,
        zoom,
        setZoom,
        terminalTheme,
        setTerminalTheme,
        resolvedTerminalTheme,
        hotkeys,
        setHotkey,
        resetHotkey,
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
