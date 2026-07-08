import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useState,
} from "react"
import type { ReactNode } from "react"

const FONT_STORAGE_KEY = "skipo.terminal.font"
const THEME_STORAGE_KEY = "skipo.appearance.theme"
const ZOOM_STORAGE_KEY = "skipo.appearance.zoom"
const TERMINAL_THEME_STORAGE_KEY = "skipo.appearance.terminalTheme"

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
}

const SettingsContext = createContext<SettingsValue | null>(null)

export function SettingsProvider({ children }: { children: ReactNode }) {
  const [font, setFontState] = useState<string>(
    () => localStorage.getItem(FONT_STORAGE_KEY) ?? DEFAULT_FONT,
  )
  const [theme, setThemeState] = useState<Theme>(readTheme)
  const [resolvedTheme, setResolvedTheme] = useState<ResolvedTheme>("light")
  const [zoom, setZoomState] = useState<number>(readZoom)
  const [terminalTheme, setTerminalThemeState] =
    useState<TerminalTheme>(readTerminalTheme)

  const setFont = useCallback((next: string) => {
    setFontState(next)
    localStorage.setItem(FONT_STORAGE_KEY, next)
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

  const setTerminalTheme = useCallback((next: TerminalTheme) => {
    setTerminalThemeState(next)
    localStorage.setItem(TERMINAL_THEME_STORAGE_KEY, next)
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

  // Scale the whole app. `zoom` reflows layout (unlike transform: scale).
  // ponytail: this also scales the terminal canvas (slightly soft off 100%);
  // zoom a chrome-only wrapper excluding TerminalHost if that becomes an issue.
  useEffect(() => {
    document.documentElement.style.zoom = String(zoom)
  }, [zoom])

  // Ctrl/Cmd +/-/0 zoom, but only when focus is outside a terminal — the
  // terminal owns those chords for its own bindings.
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (!(event.ctrlKey || event.metaKey)) return
      if (document.activeElement?.closest("[data-terminal]")) return
      if (event.key === "+" || event.key === "=") {
        event.preventDefault()
        setZoom(zoom + ZOOM_STEP)
      } else if (event.key === "-") {
        event.preventDefault()
        setZoom(zoom - ZOOM_STEP)
      } else if (event.key === "0") {
        event.preventDefault()
        setZoom(DEFAULT_ZOOM)
      }
    }
    window.addEventListener("keydown", onKey)
    return () => window.removeEventListener("keydown", onKey)
  }, [zoom, setZoom])

  const resolvedTerminalTheme: ResolvedTheme =
    terminalTheme === "match" ? resolvedTheme : terminalTheme

  return (
    <SettingsContext.Provider
      value={{
        font,
        setFont,
        theme,
        setTheme,
        resolvedTheme,
        zoom,
        setZoom,
        terminalTheme,
        setTerminalTheme,
        resolvedTerminalTheme,
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
