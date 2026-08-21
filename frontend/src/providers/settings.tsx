import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from "react"
import type { ReactNode } from "react"
import {
  DEFAULT_HOTKEYS,
  isRecordingTarget,
  loadHotkeys,
  saveHotkeys,
  type HotkeyBinding,
  type HotkeyId,
  type Hotkeys,
} from "@/lib/hotkeys"
import { zoomIntent } from "@/lib/terminal/zoom-keys"
import { isMac } from "@/lib/platform"
import {
  parseBoolPref,
  parseNumberPref,
  parseOptionalBoolPref,
  readPref,
  writePref,
} from "@/lib/prefs"
import { Store, Themes as ThemeRPC } from "@/lib/rpc"
import {
  adoptStoredSelections,
  applyAppTheme,
  BUNDLED_THEMES,
  DARK_THEME_SCHEME,
  DEFAULT_TERMINAL_THEME,
  DEFAULT_THEME,
  mergeImportedThemes,
  mergeThemes,
  reconcileThemeSelections,
  resolveTerminalTheme,
  resolveTheme,
  selectionsAfterThemeRemoval,
  SYSTEM_THEME,
} from "@/lib/themes"
import type { TerminalTheme, Theme, ResolvedTheme } from "@/lib/themes"
import type { ThemeDefinition, ThemeGitInstallResult, ThemeImportResult } from "@/lib/api-types"

export type { TerminalTheme, Theme, ResolvedTheme } from "@/lib/themes"
export { DEFAULT_TERMINAL_THEME, DEFAULT_THEME } from "@/lib/themes"

const FONT_STORAGE_KEY = "lich.terminal.font"
const TERMINAL_FONT_SIZE_STORAGE_KEY = "lich.terminal.fontSize"
const THEME_STORAGE_KEY = "lich.appearance.theme"
const ZOOM_STORAGE_KEY = "lich.appearance.zoom"
const TERMINAL_THEME_STORAGE_KEY = "lich.appearance.terminalTheme"
const CONTEXT_USAGE_STORAGE_KEY = "lich.footer.contextUsage"
const COST_BUDGET_STORAGE_KEY = "lich.footer.costBudget"
const DESKTOP_NOTIFICATIONS_STORAGE_KEY = "lich.notifications.desktop"
const FINISHED_TURN_NOTIFICATIONS_STORAGE_KEY = "lich.notifications.finishedTurn"

// The workspace keys the two theme selections actually live under. The
// localStorage pair above keeps them too, but only as the cache the first frame
// paints from: it sits in the Chromium profile, which Chromium recreates from
// scratch when its storage comes back damaged — a window killed rather than
// closed (#209) is how that happened — taking every `lich.*` pref with it
// (#236). The database is the same store the sessions survive in, and it is
// where the "what's new" flag went for the same reason (#199).
const THEME_SETTING_KEY = "appearance.theme"
const TERMINAL_THEME_SETTING_KEY = "appearance.terminalTheme"
// The selections answer for this install, not for a project.
const GLOBAL_SCOPE = ""

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

// Both preferences are read raw: the stored id is an open value, and the only
// check worth making is membership, which reconcileThemeSelections applies once
// the theme list lands. Until then an unknown id resolves to the system theme.
const readTheme = (): Theme => readPref(THEME_STORAGE_KEY) ?? DEFAULT_THEME

const readTerminalTheme = (): TerminalTheme =>
  readPref(TERMINAL_THEME_STORAGE_KEY) ?? DEFAULT_TERMINAL_THEME

const readZoom = (): number => parseNumberPref(readPref(ZOOM_STORAGE_KEY), DEFAULT_ZOOM, clampZoom)

// Default on: the footer context readout shows unless the user turned it off.
const readContextUsage = (): boolean => parseBoolPref(readPref(CONTEXT_USAGE_STORAGE_KEY), true)

// clampCostBudget bounds a spend ceiling to something a cost readout can be
// compared against: never negative, and no finer than the cent the readout
// prints. Anything that is not a number at all is no ceiling, which is what an
// emptied input has to mean — the alternative is a NaN budget that colours the
// readout forever.
function clampCostBudget(value: number): number {
  if (!Number.isFinite(value)) {
    return 0
  }
  return Math.round(Math.max(0, value) * 100) / 100
}

// The spend ceiling is off by default and reads as 0 for a stored 0 — the value
// is a magnitude, so parseNumberPref already answers the fallback for it.
const readCostBudget = (): number =>
  parseNumberPref(readPref(COST_BUDGET_STORAGE_KEY), 0, clampCostBudget)

// No default: a desktop notification interrupts the user outside the app, so it
// is the one preference lich asks about instead of assuming. null means the
// question has not been put yet — see the opt-in dialog.
const readDesktopNotifications = (): boolean | null =>
  parseOptionalBoolPref(readPref(DESKTOP_NOTIFICATIONS_STORAGE_KEY))

// Off, and its own key: a finished turn is a weaker reason to interrupt someone
// than a session blocked on them, so an install that already answered the opt-in
// above never inherits it. Turning this on is an explicit act, hence a plain
// flag with no undecided state — the dialog is not put for this channel.
const readFinishedTurnNotifications = (): boolean =>
  parseBoolPref(readPref(FINISHED_TURN_NOTIFICATIONS_STORAGE_KEY), false)

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
  importTheme: (path: string, overwrite: boolean) => Promise<ThemeImportResult>
  /** Install every theme of a repository, versioned by its manifest. */
  installThemesFromGit: (url: string, overwrite: boolean) => Promise<ThemeGitInstallResult>
  /** Re-clone the repository a theme came from and install a newer manifest. */
  updateThemeFromGit: (id: string) => Promise<ThemeGitInstallResult>
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
  /** Configurable keyboard shortcuts and terminal translations, keyed by action. */
  hotkeys: Hotkeys
  setHotkey: (id: HotkeyId, combo: HotkeyBinding) => void
  resetHotkey: (id: HotkeyId) => void
  /** Whether the footer shows the active session's context-window usage. */
  showContextUsage: boolean
  setShowContextUsage: (show: boolean) => void
  /** Spend ceiling in USD the footer cost readout colours against; 0 is none. */
  costBudget: number
  setCostBudget: (usd: number) => void
  /** Whether a session needing input notifies the desktop while the lich window
   * is unfocused. null until the user has answered the opt-in dialog. */
  desktopNotifications: boolean | null
  setDesktopNotifications: (enabled: boolean) => void
  /** Whether a session that finished its turn notifies the desktop while the
   * lich window is unfocused. Independent of the opt-in above, and off until
   * the user turns it on. */
  finishedTurnNotifications: boolean
  setFinishedTurnNotifications: (enabled: boolean) => void
}

const SettingsContext = createContext<SettingsValue | null>(null)

export function SettingsProvider({ children }: { children: ReactNode }) {
  const [font, setFontState] = useState<string>(readFont)
  const [terminalFontSize, setTerminalFontSizeState] = useState<number>(readTerminalFontSize)
  const [themes, setThemes] = useState<ThemeDefinition[]>(BUNDLED_THEMES)
  const [themesLoaded, setThemesLoaded] = useState(false)
  const [theme, setThemeState] = useState<Theme>(readTheme)
  const [resolvedTheme, setResolvedTheme] = useState<ResolvedTheme>(BUNDLED_THEMES[0])
  const [zoom, setZoomState] = useState<number>(readZoom)
  const [terminalTheme, setTerminalThemeState] = useState<TerminalTheme>(readTerminalTheme)
  const [hotkeys, setHotkeys] = useState<Hotkeys>(loadHotkeys)
  const [showContextUsage, setShowContextUsageState] = useState<boolean>(readContextUsage)
  const [costBudget, setCostBudgetState] = useState<number>(readCostBudget)
  const [desktopNotifications, setDesktopNotificationsState] = useState<boolean | null>(
    readDesktopNotifications,
  )
  const [finishedTurnNotifications, setFinishedTurnNotificationsState] = useState<boolean>(
    readFinishedTurnNotifications,
  )

  // A selection the user (or a reconcile) has already written must not be
  // overwritten by the stored one still in flight, so the load below stands down
  // once anything has been persisted.
  const selectionsTouched = useRef(false)

  const persistTheme = useCallback((next: Theme) => {
    selectionsTouched.current = true
    writePref(THEME_STORAGE_KEY, next)
    // A failed write costs the selection only if the profile is later recreated
    // — the cache answers every ordinary launch — so it stays quiet.
    void Store.SetSetting(THEME_SETTING_KEY, GLOBAL_SCOPE, next).catch(() => {})
  }, [])

  const persistTerminalTheme = useCallback((next: TerminalTheme) => {
    selectionsTouched.current = true
    writePref(TERMINAL_THEME_STORAGE_KEY, next)
    void Store.SetSetting(TERMINAL_THEME_SETTING_KEY, GLOBAL_SCOPE, next).catch(() => {})
  }, [])

  // The stored selections land after the first paint, which the cache has
  // already answered for: a theme that is only in the database repaints once,
  // and a custom one waits for the theme list either way.
  useEffect(() => {
    let cancelled = false
    const cached = { theme: readTheme(), terminalTheme: readTerminalTheme() }
    Promise.all([
      Store.GetSetting(THEME_SETTING_KEY, GLOBAL_SCOPE),
      Store.GetSetting(TERMINAL_THEME_SETTING_KEY, GLOBAL_SCOPE),
    ])
      .then(([storedTheme, storedTerminalTheme]) => {
        if (cancelled || selectionsTouched.current) return
        const { selections, persist } = adoptStoredSelections(
          { theme: storedTheme, terminalTheme: storedTerminalTheme },
          cached,
        )
        if (selections.theme !== cached.theme) {
          setThemeState(selections.theme)
          writePref(THEME_STORAGE_KEY, selections.theme)
        }
        if (selections.terminalTheme !== cached.terminalTheme) {
          setTerminalThemeState(selections.terminalTheme)
          writePref(TERMINAL_THEME_STORAGE_KEY, selections.terminalTheme)
        }
        if (persist) {
          persistTheme(selections.theme)
          persistTerminalTheme(selections.terminalTheme)
        }
      })
      .catch((error) => {
        console.warn("[settings] failed to load the stored theme selections", error)
      })
    return () => {
      cancelled = true
    }
  }, [persistTheme, persistTerminalTheme])

  useEffect(() => {
    let cancelled = false
    ThemeRPC.List()
      .then((loaded) => {
        if (!cancelled) {
          setThemes(mergeThemes(loaded))
          setThemesLoaded(true)
        }
      })
      .catch((error) => {
        console.warn("[settings] failed to load custom themes", error)
      })
    return () => {
      cancelled = true
    }
  }, [])

  useEffect(() => {
    if (!themesLoaded) return
    const reconciled = reconcileThemeSelections({ theme, terminalTheme }, themes)
    if (reconciled.theme !== theme) {
      setThemeState(reconciled.theme)
      persistTheme(reconciled.theme)
    }
    if (reconciled.terminalTheme !== terminalTheme) {
      setTerminalThemeState(reconciled.terminalTheme)
      persistTerminalTheme(reconciled.terminalTheme)
    }
  }, [theme, terminalTheme, themes, themesLoaded, persistTheme, persistTerminalTheme])

  const setFont = useCallback((next: string) => {
    setFontState(next)
    writePref(FONT_STORAGE_KEY, next)
  }, [])

  const setTerminalFontSize = useCallback((next: number) => {
    const clamped = clampTerminalFontSize(next)
    setTerminalFontSizeState(clamped)
    writePref(TERMINAL_FONT_SIZE_STORAGE_KEY, clamped)
  }, [])

  const setTheme = useCallback(
    (next: Theme) => {
      setThemeState(next)
      persistTheme(next)
    },
    [persistTheme],
  )

  // Neither of these persists: the zoom is mirrored to localStorage by the
  // effect that applies it, so the one updater that has to be functional (see
  // zoomBy) stays free of the side effect React calls twice under StrictMode.
  const setZoom = useCallback((next: number) => {
    setZoomState(clampZoom(next))
  }, [])

  // zoomBy applies a relative step off the latest value so rapid wheel ticks
  // accumulate instead of collapsing to a single step between renders.
  const zoomBy = useCallback((delta: number) => {
    setZoomState((prev) => clampZoom(prev + delta))
  }, [])

  const setTerminalTheme = useCallback(
    (next: TerminalTheme) => {
      setTerminalThemeState(next)
      persistTerminalTheme(next)
    },
    [persistTerminalTheme],
  )

  const importTheme = useCallback(
    async (path: string, overwrite: boolean) => {
      const result = await ThemeRPC.Import(path, overwrite)
      if (!result.needsOverwrite) {
        setThemes((prev) => mergeImportedThemes(prev, [result.theme]))
        setTheme(result.theme.id)
      }
      return result
    },
    [setTheme],
  )

  // A pack installs as a whole, so its first theme is what gets selected —
  // selecting nothing would leave the user with no sign the install landed.
  const adoptInstalledThemes = useCallback(
    (installed: readonly ThemeDefinition[], select: boolean) => {
      if (installed.length === 0) return
      setThemes((prev) => mergeImportedThemes(prev, installed))
      if (select) {
        setTheme(installed[0].id)
      }
    },
    [setTheme],
  )

  const installThemesFromGit = useCallback(
    async (url: string, overwrite: boolean) => {
      const result = await ThemeRPC.InstallFromGit(url, overwrite)
      adoptInstalledThemes(result.themes ?? [], true)
      return result
    },
    [adoptInstalledThemes],
  )

  const updateThemeFromGit = useCallback(
    async (id: string) => {
      const result = await ThemeRPC.UpdateFromGit(id)
      // An update repaints themes the user may already be looking at, but it
      // must not move the selection off the one they picked.
      adoptInstalledThemes(result.themes ?? [], false)
      return result
    },
    [adoptInstalledThemes],
  )

  const removeTheme = useCallback(
    async (id: string) => {
      await ThemeRPC.Remove(id)
      setThemes((prev) => mergeThemes(prev.filter((item) => item.id !== id)))
      const next = selectionsAfterThemeRemoval(id, { theme, terminalTheme })
      if (next.theme !== theme) {
        setThemeState(next.theme)
        persistTheme(next.theme)
      }
      if (next.terminalTheme !== terminalTheme) {
        setTerminalThemeState(next.terminalTheme)
        persistTerminalTheme(next.terminalTheme)
      }
    },
    [theme, terminalTheme, persistTheme, persistTerminalTheme],
  )

  const setHotkey = useCallback((id: HotkeyId, combo: HotkeyBinding) => {
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

  const setCostBudget = useCallback((next: number) => {
    const clamped = clampCostBudget(next)
    setCostBudgetState(clamped)
    writePref(COST_BUDGET_STORAGE_KEY, clamped)
  }, [])

  // Writing either answer settles the question for good: a refusal is stored,
  // not left absent, so the dialog is put once and the toggle owns it after.
  const setDesktopNotifications = useCallback((next: boolean) => {
    setDesktopNotificationsState(next)
    writePref(DESKTOP_NOTIFICATIONS_STORAGE_KEY, next)
  }, [])

  const setFinishedTurnNotifications = useCallback((next: boolean) => {
    setFinishedTurnNotificationsState(next)
    writePref(FINISHED_TURN_NOTIFICATIONS_STORAGE_KEY, next)
  }, [])

  // Apply the resolved theme's CSS variables and toggle `.dark` for existing
  // dark variants. For "system", follow the OS scheme and keep following it
  // live.
  useEffect(() => {
    const media = window.matchMedia("(prefers-color-scheme: dark)")
    const apply = () => {
      const resolved = resolveTheme(theme, themes, media.matches)
      applyAppTheme(resolved, document.documentElement)
      document.documentElement.classList.toggle(
        DARK_THEME_SCHEME,
        resolved.scheme === DARK_THEME_SCHEME,
      )
      setResolvedTheme(resolved)
    }
    apply()
    if (theme !== SYSTEM_THEME) return
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
  //
  // Also where the value is persisted, so both setters can stay pure.
  useEffect(() => {
    document.documentElement.style.fontSize = `${zoom * 100}%`
    writePref(ZOOM_STORAGE_KEY, zoom)
  }, [zoom])

  // Keyboard zoom is configurable; Ctrl/Cmd+wheel remains a gesture rather
  // than a key binding. The browser accelerator guard independently prevents
  // Chromium zoom, so a disabled or rebound old chord reaches the TUI without
  // changing either lich's zoom or Chromium's.
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (isRecordingTarget(event)) return
      const intent = zoomIntent(event, hotkeys, isMac)
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
  }, [hotkeys, zoomBy, setZoom])

  const resolvedTerminalTheme = resolveTerminalTheme(terminalTheme, resolvedTheme, themes)

  // Memoised for the same reason ProjectsProvider is: a literal here hands every
  // consumer a new object on every render of this provider, and one of those
  // consumers is TerminalView — one per spawned session. A Ctrl+wheel zoom is a
  // burst of state changes, so the un-memoised value re-rendered every open
  // terminal on every wheel tick.
  const value = useMemo(
    () => ({
      font,
      setFont,
      terminalFontSize,
      setTerminalFontSize,
      themes,
      theme,
      setTheme,
      importTheme,
      installThemesFromGit,
      updateThemeFromGit,
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
      costBudget,
      setCostBudget,
      desktopNotifications,
      setDesktopNotifications,
      finishedTurnNotifications,
      setFinishedTurnNotifications,
    }),
    [
      font,
      setFont,
      terminalFontSize,
      setTerminalFontSize,
      themes,
      theme,
      setTheme,
      importTheme,
      installThemesFromGit,
      updateThemeFromGit,
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
      costBudget,
      setCostBudget,
      desktopNotifications,
      setDesktopNotifications,
      finishedTurnNotifications,
      setFinishedTurnNotifications,
    ],
  )

  return <SettingsContext.Provider value={value}>{children}</SettingsContext.Provider>
}

export function useSettings(): SettingsValue {
  const ctx = useContext(SettingsContext)
  if (!ctx) {
    throw new Error("useSettings must be used within a SettingsProvider")
  }
  return ctx
}
