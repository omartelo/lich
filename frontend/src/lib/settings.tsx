import { createContext, useCallback, useContext, useState } from "react"
import type { ReactNode } from "react"

const FONT_STORAGE_KEY = "skipo.terminal.font"

// DEFAULT_FONT is the bundled FiraCode Nerd Font Mono. It is not installed via
// fontconfig, so it must be offered explicitly alongside the system fonts.
export const DEFAULT_FONT = "FiraCode Nerd Font Mono"

interface SettingsValue {
  /** Terminal font family, applied globally across all project terminals. */
  font: string
  setFont: (font: string) => void
}

const SettingsContext = createContext<SettingsValue | null>(null)

export function SettingsProvider({ children }: { children: ReactNode }) {
  const [font, setFontState] = useState<string>(
    () => localStorage.getItem(FONT_STORAGE_KEY) ?? DEFAULT_FONT,
  )

  const setFont = useCallback((next: string) => {
    setFontState(next)
    localStorage.setItem(FONT_STORAGE_KEY, next)
  }, [])

  return (
    <SettingsContext.Provider value={{ font, setFont }}>
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
