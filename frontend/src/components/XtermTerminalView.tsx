import { useEffect, useRef } from "react"
import { Terminal } from "@xterm/xterm"
import { WebglAddon } from "@xterm/addon-webgl"
import { FitAddon } from "@xterm/addon-fit"
import { Terminal as Service } from "@/lib/rpc"
import { onAppEvent } from "@/lib/app-events"
import { ensureTransport, onSessionData, sendInput } from "@/lib/term-transport"
import { recordChunk } from "@/lib/term-perf"
import { useSettings } from "@/lib/settings"
import {
  decodeBase64,
  ensureFontLoaded,
  FONT_SIZE,
  TERMINAL_COLORS,
  type TerminalViewProps,
} from "@/components/TerminalView"
import "@xterm/xterm/css/xterm.css"

// Proof of concept: xterm.js 6 + the WebGL renderer, benchmarking GPU text
// against the patched ghostty-web canvas-2d pipeline (flag: lich.xtermPoc,
// see lib/xterm-poc.ts). Same PTY, same transport, same term-perf metrics —
// only the terminal emulator differs. Deliberately NOT at feature parity:
// none of the ghostty WebKitGTK workarounds (Shift+Tab/Alt chords, SGR wheel
// forwarding, DOM guard, selection toast, link opening), no live font/theme
// switching, no hidden-canvas release. Disposable: delete this file and the
// flag once the migration decision lands.

const DATA_EVENT_PREFIX = "terminal:data:"
const EXIT_EVENT_PREFIX = "terminal:exit:"
const REFIT_DEBOUNCE_MS = 100

export function XtermTerminalView({ sessionId, projectId, cwd, kind, visible }: TerminalViewProps) {
  const { font, resolvedTerminalTheme } = useSettings()
  const containerRef = useRef<HTMLDivElement | null>(null)
  const termRef = useRef<Terminal | null>(null)
  const fitRef = useRef<FitAddon | null>(null)
  const visibleRef = useRef(visible)
  const fontRef = useRef(font)
  const themeRef = useRef(resolvedTerminalTheme)
  visibleRef.current = visible
  fontRef.current = font
  themeRef.current = resolvedTerminalTheme

  useEffect(() => {
    const container = containerRef.current
    if (!container) {
      return
    }

    let disposed = false
    const cleanups: Array<() => void> = []

    void (async () => {
      await ensureFontLoaded(fontRef.current)
      if (disposed) {
        return
      }

      const term = new Terminal({
        fontSize: FONT_SIZE,
        fontFamily: `"${fontRef.current}", monospace`,
        cursorBlink: true,
        scrollback: 5000,
        allowProposedApi: true,
        theme: TERMINAL_COLORS[themeRef.current],
      })
      const fitAddon = new FitAddon()
      term.loadAddon(fitAddon)
      term.open(container)

      // WebGL is the whole point of the POC; context loss falls back to the
      // DOM renderer (that result alone would kill the migration).
      const webgl = new WebglAddon()
      webgl.onContextLoss(() => {
        console.warn("[xterm-poc] WebGL context lost, DOM renderer from here on")
        webgl.dispose()
      })
      term.loadAddon(webgl)

      fitAddon.fit()
      termRef.current = term
      fitRef.current = fitAddon
      ensureTransport()

      const writeInput = (data: string) => {
        if (!sendInput(sessionId, data)) {
          void Service.Write(sessionId, data)
        }
      }
      const dataInput = term.onData(writeInput)
      const resizeInput = term.onResize(({ cols, rows }) => {
        if (visibleRef.current) {
          void Service.Resize(sessionId, cols, rows)
        }
      })
      cleanups.push(() => dataInput.dispose(), () => resizeInput.dispose())

      // xterm parses asynchronously (internal write buffer), so writeMs here
      // includes queue wait + parse, where ghostty's write is a synchronous
      // WASM parse. Frame cost lands in term-perf's shared rAF/stall metrics.
      const offData = onAppEvent(DATA_EVENT_PREFIX + sessionId, (data) => {
        const t0 = performance.now()
        const bytes = decodeBase64(data as string)
        const t1 = performance.now()
        term.write(bytes, () => recordChunk(t1 - t0, performance.now() - t1, bytes.length))
      })
      const offWsData = onSessionData(sessionId, (payload) => {
        const t0 = performance.now()
        term.write(payload, () => recordChunk(0, performance.now() - t0, payload.length))
      })
      const offExit = onAppEvent(EXIT_EVENT_PREFIX + sessionId, () => {
        term.write("\r\n[process exited]\r\n")
      })
      cleanups.push(offData, offWsData, offExit)

      let refitTimer = 0
      const resizeObserver = new ResizeObserver(() => {
        window.clearTimeout(refitTimer)
        refitTimer = window.setTimeout(() => {
          if (visibleRef.current) {
            fitRef.current?.fit()
          }
        }, REFIT_DEBOUNCE_MS)
      })
      resizeObserver.observe(container)
      cleanups.push(() => {
        window.clearTimeout(refitTimer)
        resizeObserver.disconnect()
      })

      await Service.Start(sessionId, projectId, cwd, kind, term.cols, term.rows)
      if (visibleRef.current) {
        term.focus()
      } else {
        void Service.SetVisible(sessionId, false)
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
  }, [sessionId, projectId, cwd, kind])

  // Visibility: backend coalescing + refit + focus. No render-loop pause —
  // xterm only paints on damage, so a hidden, idle xterm costs no frames.
  useEffect(() => {
    const term = termRef.current
    if (!term) {
      return
    }
    if (!visible) {
      void Service.SetVisible(sessionId, false)
      return
    }
    void Service.SetVisible(sessionId, true)
    fitRef.current?.fit()
    void Service.Resize(sessionId, term.cols, term.rows)
    term.focus()
  }, [visible, sessionId])

  return <div ref={containerRef} data-terminal className="h-full w-full" />
}
