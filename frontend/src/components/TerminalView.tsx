import { useEffect, useRef } from "react"
import { init, Terminal as Ghostty, FitAddon } from "ghostty-web"
import { Events } from "@wailsio/runtime"
import { Service } from "../../bindings/github.com/skipodotdev/skipo/internals/terminal"
import { patchBlockGlyphs } from "@/lib/block-glyphs"
import { patchFontMetrics } from "@/lib/font-metrics"
import { useSettings } from "@/lib/settings"
import type { ResolvedTheme } from "@/lib/settings"

// Event name prefixes mirror the backend (internals/terminal); the concrete
// event carries the session ID as a suffix.
const DATA_EVENT_PREFIX = "terminal:data:"
const EXIT_EVENT_PREFIX = "terminal:exit:"

const FONT_SIZE = 14

// Terminal color schemes. Light keeps a high-contrast foreground so CLI output
// stays legible against the pale background.
const TERMINAL_COLORS: Record<ResolvedTheme, { background: string; foreground: string }> = {
  dark: { background: "#06070f", foreground: "#e5e7eb" },
  light: { background: "#ffffff", foreground: "#1f2328" },
}

// init loads the WASM module once and is shared across every terminal instance.
let initPromise: Promise<void> | null = null
function ensureInit(): Promise<void> {
  return (initPromise ??= init())
}

// ensureFontLoaded blocks until a font is available. The canvas renderer only
// draws with fonts already loaded in the document. Bundled fonts (@font-face)
// need this; system fonts resolve through the webview's fontconfig and load
// as no-ops, so failures are ignored.
async function ensureFontLoaded(font: string): Promise<void> {
  try {
    await Promise.all([
      document.fonts.load(`${FONT_SIZE}px "${font}"`),
      document.fonts.load(`bold ${FONT_SIZE}px "${font}"`),
    ])
  } catch {
    // Fall back to the system monospace face.
  }
}

// decodeBase64 turns the base64 PTY payload back into bytes. The backend encodes
// output so multi-byte UTF-8 sequences survive the JSON event bridge intact.
function decodeBase64(data: string): Uint8Array {
  const binary = atob(data)
  const bytes = new Uint8Array(binary.length)
  for (let i = 0; i < binary.length; i++) {
    bytes[i] = binary.charCodeAt(i)
  }
  return bytes
}

interface TerminalViewProps {
  sessionId: string
  projectId: string
  cwd: string
  visible: boolean
}

export function TerminalView({ sessionId, projectId, cwd, visible }: TerminalViewProps) {
  const { font, resolvedTerminalTheme } = useSettings()
  const containerRef = useRef<HTMLDivElement | null>(null)
  const termRef = useRef<Ghostty | null>(null)
  const fitRef = useRef<FitAddon | null>(null)
  // Latest values for use inside long-lived async callbacks / listeners.
  const visibleRef = useRef(visible)
  const fontRef = useRef(font)
  const themeRef = useRef(resolvedTerminalTheme)
  visibleRef.current = visible
  fontRef.current = font
  themeRef.current = resolvedTerminalTheme

  // Create the terminal and its PTY session once per session id. The session
  // runs in the background regardless of visibility and is only torn down when
  // it is closed (this component unmounts) — never on navigation.
  useEffect(() => {
    const container = containerRef.current
    if (!container) {
      return
    }

    let disposed = false
    const cleanups: Array<() => void> = []

    // Refit the grid when the container settles at a new size — window resize,
    // sidebar drag, zoom. fit() is expensive (full WASM buffer + canvas realloc
    // + redraw, for every mounted terminal) and each PTY resize makes the running
    // TUI repaint, so refitting mid-drag starves pointer events and the drag lags
    // behind the cursor. A trailing debounce keeps drags fluid: no terminal work
    // while the size is still changing, one refit once it stops.
    // Guard on integer dimensions: fit() nudges the canvas, which can make the
    // observer re-fire at sub-pixel deltas and loop into a flicker.
    // Only the visible terminal pushes its new size to the PTY; hidden ones refit
    // but sync their PTY when shown.
    const REFIT_DEBOUNCE_MS = 100
    let lastWidth = 0
    let lastHeight = 0
    let refitTimer = 0
    const scheduleRefit = (entries: ResizeObserverEntry[]) => {
      const rect = entries[0]?.contentRect
      if (rect) {
        const width = Math.round(rect.width)
        const height = Math.round(rect.height)
        if (width === lastWidth && height === lastHeight) {
          return
        }
        lastWidth = width
        lastHeight = height
      }
      window.clearTimeout(refitTimer)
      refitTimer = window.setTimeout(() => {
        const term = termRef.current
        if (!term) {
          return
        }
        fitRef.current?.fit()
        if (visibleRef.current) {
          void Service.Resize(sessionId, term.cols, term.rows)
        }
      }, REFIT_DEBOUNCE_MS)
    }

    void (async () => {
      await ensureInit()
      await ensureFontLoaded(fontRef.current)
      if (disposed) {
        return
      }

      const term = new Ghostty({
        fontSize: FONT_SIZE,
        fontFamily: `"${fontRef.current}", monospace`,
        cursorBlink: true,
        scrollback: 5000,
        theme: TERMINAL_COLORS[themeRef.current],
      })
      const fit = new FitAddon()
      term.loadAddon(fit)
      term.open(container)
      if (term.renderer) {
        patchBlockGlyphs(term.renderer)
        patchFontMetrics(term.renderer)
      }
      fit.fit()
      termRef.current = term
      fitRef.current = fit

      const dataInput = term.onData((data) => Service.Write(sessionId, data))
      const resizeInput = term.onResize(({ cols, rows }) => {
        if (visibleRef.current) {
          void Service.Resize(sessionId, cols, rows)
        }
      })
      cleanups.push(() => dataInput.dispose(), () => resizeInput.dispose())

      const offData = Events.On(DATA_EVENT_PREFIX + sessionId, (event) => {
        term.write(decodeBase64(event.data as string))
      })
      const offExit = Events.On(EXIT_EVENT_PREFIX + sessionId, () => {
        term.write("\r\n[process exited]\r\n")
      })
      cleanups.push(offData, offExit)

      const resizeObserver = new ResizeObserver(scheduleRefit)
      resizeObserver.observe(container)
      cleanups.push(() => {
        window.clearTimeout(refitTimer)
        resizeObserver.disconnect()
      })

      await Service.Start(sessionId, projectId, cwd, term.cols, term.rows)
      if (visibleRef.current) {
        term.focus()
      }
    })()

    return () => {
      disposed = true
      for (const cleanup of cleanups) {
        cleanup()
      }
      void Service.Close(sessionId)
      termRef.current?.dispose()
      termRef.current = null
      fitRef.current = null
    }
  }, [sessionId, projectId, cwd])

  // Apply a font change live to the running terminal.
  useEffect(() => {
    const term = termRef.current
    if (!term) {
      return
    }
    void (async () => {
      await ensureFontLoaded(font)
      term.renderer?.setFontFamily(font)
      // Font metrics changed: recompute the grid and sync the visible PTY.
      fitRef.current?.fit()
      if (visibleRef.current) {
        void Service.Resize(sessionId, term.cols, term.rows)
      }
    })()
  }, [font, sessionId])

  // Apply a terminal theme change live to the running terminal.
  useEffect(() => {
    termRef.current?.renderer?.setTheme(TERMINAL_COLORS[resolvedTerminalTheme])
  }, [resolvedTerminalTheme])

  // On becoming visible, refit (window may have resized while hidden), sync the
  // PTY size and focus.
  useEffect(() => {
    if (!visible) {
      return
    }
    const term = termRef.current
    if (!term) {
      return
    }
    fitRef.current?.fit()
    void Service.Resize(sessionId, term.cols, term.rows)
    term.focus()
  }, [visible, sessionId])

  return <div ref={containerRef} data-terminal className="h-full w-full" />
}
