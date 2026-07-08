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

// DEFAULT_FONT is the bundled FiraCode Nerd Font Mono. It is not installed via
// fontconfig, so it must be offered explicitly alongside the system fonts.
export const DEFAULT_FONT = "FiraCode Nerd Font Mono"

// THEMES drives both the persisted value and the Appearance picker options.
// "system" follows the OS color scheme live.
export const THEMES = ["system", "light", "dark"] as const
export type Theme = (typeof THEMES)[number]
export const DEFAULT_THEME: Theme = "system"

function readTheme(): Theme {
  const stored = localStorage.getItem(THEME_STORAGE_KEY)
  return THEMES.includes(stored as Theme) ? (stored as Theme) : DEFAULT_THEME
}

interface SettingsValue {
  /** Terminal font family, applied globally across all project terminals. */
  font: string
  setFont: (font: string) => void
  /** Color theme applied to the whole app via the `.dark` class on <html>. */
  theme: Theme
  setTheme: (theme: Theme) => void
}

const SettingsContext = createContext<SettingsValue | null>(null)

export function SettingsProvider({ children }: { children: ReactNode }) {
  const [font, setFontState] = useState<string>(
    () => localStorage.getItem(FONT_STORAGE_KEY) ?? DEFAULT_FONT,
  )
  const [theme, setThemeState] = useState<Theme>(readTheme)

  const setFont = useCallback((next: string) => {
    setFontState(next)
    localStorage.setItem(FONT_STORAGE_KEY, next)
  }, [])

  const setTheme = useCallback((next: Theme) => {
    setThemeState(next)
    localStorage.setItem(THEME_STORAGE_KEY, next)
  }, [])

  // Toggle the `.dark` class on <html>. For "system", follow the OS scheme and
  // keep following it live while that mode is selected.
  useEffect(() => {
    const media = window.matchMedia("(prefers-color-scheme: dark)")
    const apply = () => {
      const dark = theme === "dark" || (theme === "system" && media.matches)
      document.documentElement.classList.toggle("dark", dark)
    }
    apply()
    if (theme !== "system") return
    media.addEventListener("change", apply)
    return () => media.removeEventListener("change", apply)
  }, [theme])

  return (
    <SettingsContext.Provider value={{ font, setFont, theme, setTheme }}>
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
