import { useEffect, useRef } from "react"
import { init, Terminal as Ghostty, FitAddon } from "ghostty-web"
import { Events } from "@wailsio/runtime"
import { Service } from "../../bindings/github.com/skipodotdev/skipo/internals/terminal"
import { useSettings } from "@/lib/settings"

// Event name prefixes mirror the backend (internals/terminal); the concrete
// event carries the project ID as a suffix.
const DATA_EVENT_PREFIX = "terminal:data:"
const EXIT_EVENT_PREFIX = "terminal:exit:"

const FONT_SIZE = 14

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
  projectId: string
  visible: boolean
}

export function TerminalView({ projectId, visible }: TerminalViewProps) {
  const { font } = useSettings()
  const containerRef = useRef<HTMLDivElement | null>(null)
  const termRef = useRef<Ghostty | null>(null)
  const fitRef = useRef<FitAddon | null>(null)
  // Latest values for use inside long-lived async callbacks / listeners.
  const visibleRef = useRef(visible)
  const fontRef = useRef(font)
  visibleRef.current = visible
  fontRef.current = font

  // Create the terminal and its PTY session once per project. The session runs
  // in the background regardless of visibility and is only torn down when the
  // project is closed (this component unmounts) — never on navigation.
  useEffect(() => {
    const container = containerRef.current
    if (!container) {
      return
    }

    let disposed = false
    const cleanups: Array<() => void> = []

    const onWindowResize = () => {
      const term = termRef.current
      fitRef.current?.fit()
      if (term && visibleRef.current) {
        void Service.Resize(projectId, term.cols, term.rows)
      }
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
        theme: { background: "#06070f", foreground: "#e5e7eb" },
      })
      const fit = new FitAddon()
      term.loadAddon(fit)
      term.open(container)
      fit.fit()
      termRef.current = term
      fitRef.current = fit

      const dataInput = term.onData((data) => Service.Write(projectId, data))
      const resizeInput = term.onResize(({ cols, rows }) => {
        if (visibleRef.current) {
          void Service.Resize(projectId, cols, rows)
        }
      })
      cleanups.push(() => dataInput.dispose(), () => resizeInput.dispose())

      const offData = Events.On(DATA_EVENT_PREFIX + projectId, (event) => {
        term.write(decodeBase64(event.data as string))
      })
      const offExit = Events.On(EXIT_EVENT_PREFIX + projectId, () => {
        term.write("\r\n[process exited]\r\n")
      })
      cleanups.push(offData, offExit)

      window.addEventListener("resize", onWindowResize)
      cleanups.push(() => window.removeEventListener("resize", onWindowResize))

      await Service.Start(projectId, "", term.cols, term.rows)
      if (visibleRef.current) {
        term.focus()
      }
    })()

    return () => {
      disposed = true
      for (const cleanup of cleanups) {
        cleanup()
      }
      void Service.Close(projectId)
      termRef.current?.dispose()
      termRef.current = null
      fitRef.current = null
    }
  }, [projectId])

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
        void Service.Resize(projectId, term.cols, term.rows)
      }
    })()
  }, [font, projectId])

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
    void Service.Resize(projectId, term.cols, term.rows)
    term.focus()
  }, [visible, projectId])

  return <div ref={containerRef} className="h-full w-full" />
}
