import { useEffect, useRef } from "react"
import { init, Terminal as Ghostty } from "ghostty-web"
import { Clipboard } from "@wailsio/runtime"
import { Terminal as Service } from "@/lib/rpc"
import { onAppEvent } from "@/lib/app-events"
import { toast } from "sonner"
import { copyToastMessage, COPY_TOAST_DURATION_MS } from "@/lib/copy-toast"
import { patchBlockGlyphs } from "@/lib/block-glyphs"
import { patchFontMetrics } from "@/lib/font-metrics"
import { patchGlyphAtlas } from "@/lib/glyph-atlas"
import { patchPooledGetLine } from "@/lib/getline-pool"
import { patchScrollGate, patchScrollbackCache } from "@/lib/scrollback-perf"
import { ensureTransport, onSessionData, sendInput } from "@/lib/term-transport"
import { registerLinkOpening } from "@/lib/term-links"
import { pauseRenderLoop, resumeRenderLoop } from "@/lib/render-pause"
import { releaseCanvasBacking } from "@/lib/hidden-canvas"
import { patchRowPaint } from "@/lib/row-paint"
import {
  altScreenWheelSequence,
  isTextPasteChord,
  missingKeySequence,
  sgrWheelSequence,
} from "@/lib/term-keys"
import { computeGrid } from "@/lib/term-fit"
import { isStrayTerminalChild } from "@/lib/term-dom"
import { countingCanvasFactory, instrumentRender, recordChunk } from "@/lib/term-perf"
import { useSettings } from "@/lib/settings"
import type { ResolvedTheme } from "@/lib/settings"
import type { SessionKind } from "@/lib/sessions"

// Event name prefixes mirror the backend (internal/terminal); the concrete
// event carries the session ID as a suffix.
const DATA_EVENT_PREFIX = "terminal:data:"
const EXIT_EVENT_PREFIX = "terminal:exit:"

export const FONT_SIZE = 14

// Terminal color schemes. Light keeps a high-contrast foreground so CLI output
// stays legible against the pale background. Shared with XtermTerminalView so
// the POC renders under identical visuals.
export const TERMINAL_COLORS: Record<ResolvedTheme, { background: string; foreground: string }> = {
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
export async function ensureFontLoaded(font: string): Promise<void> {
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
export function decodeBase64(data: string): Uint8Array {
  const binary = atob(data)
  const bytes = new Uint8Array(binary.length)
  for (let i = 0; i < binary.length; i++) {
    bytes[i] = binary.charCodeAt(i)
  }
  return bytes
}

// fitTerminal resizes the grid to fill the container edge to edge (replacing
// ghostty-web's FitAddon, which reserves a fixed 15px right gutter). No-op when
// metrics or size aren't ready yet, or the grid is already the right size.
function fitTerminal(term: Ghostty, container: HTMLElement): void {
  const cell = term.renderer?.getMetrics()
  if (!cell) {
    return
  }
  const grid = computeGrid(container.clientWidth, container.clientHeight, cell)
  if (grid && (grid.cols !== term.cols || grid.rows !== term.rows)) {
    term.resize(grid.cols, grid.rows)
  }
}

// pointerCell maps a wheel/mouse event to the 1-based terminal cell under the
// pointer, for the coordinates an SGR mouse report carries. Falls back to the
// top-left cell when metrics aren't ready; clamps into the grid so the report
// never names a cell outside it.
function pointerCell(
  event: WheelEvent,
  container: HTMLElement,
  renderer: Ghostty["renderer"],
  cols: number,
  rows: number,
): { col: number; row: number } {
  const cell = renderer?.getMetrics()
  if (!cell) {
    return { col: 1, row: 1 }
  }
  const rect = container.getBoundingClientRect()
  const col = Math.floor((event.clientX - rect.left) / cell.width) + 1
  const row = Math.floor((event.clientY - rect.top) / cell.height) + 1
  return {
    col: Math.max(1, Math.min(cols, col)),
    row: Math.max(1, Math.min(rows, row)),
  }
}

export interface TerminalViewProps {
  sessionId: string
  projectId: string
  cwd: string
  kind: SessionKind
  visible: boolean
}

export function TerminalView({ sessionId, projectId, cwd, kind, visible }: TerminalViewProps) {
  const { font, resolvedTerminalTheme } = useSettings()
  const containerRef = useRef<HTMLDivElement | null>(null)
  const termRef = useRef<Ghostty | null>(null)
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
    // Only the visible terminal refits and syncs its PTY; hidden ones skip the
    // work entirely and catch up via the unconditional refit on show.
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
        if (!term || !visibleRef.current) {
          return
        }
        fitTerminal(term, container)
        void Service.Resize(sessionId, term.cols, term.rows)
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
      term.open(container)
      if (term.renderer) {
        patchBlockGlyphs(term.renderer)
        patchFontMetrics(term.renderer)
        patchRowPaint(term.renderer)
        patchGlyphAtlas(term.renderer, countingCanvasFactory())
        instrumentRender(term.renderer)
        // Gate wraps last so skipped frames never reach the instrumented
        // render — term-perf keeps counting only real paints.
        patchScrollGate(term.renderer)
      }
      patchPooledGetLine(term.wasmTerm)
      patchScrollbackCache(term)
      registerLinkOpening(term)
      fitTerminal(term, container)
      termRef.current = term
      ensureTransport()

      // Input goes over the local WebSocket when it is up; otherwise the
      // Wails binding. Both land in the same PTY write on the backend.
      const writeInput = (data: string) => {
        if (!sendInput(sessionId, data)) {
          void Service.Write(sessionId, data)
        }
      }

      // Sequences ghostty-web 0.4.0 gets wrong (Shift+Tab, Alt chords) are
      // written to the PTY directly; returning true stops the terminal from
      // sending its own broken encoding. See term-keys.ts.
      term.attachCustomKeyEventHandler((event) => {
        // Ctrl+Shift+V pastes text from the clipboard. navigator.clipboard
        // needs a secure context — real in the Chromium shell (localhost),
        // absent under the wails:// origin, where the Wails clipboard picks
        // up. Plain Ctrl+V reaches the PTY as SYN through missingKeySequence,
        // like a real terminal.
        if (isTextPasteChord(event)) {
          const read =
            navigator.clipboard?.readText?.() ?? Promise.reject(new Error("no clipboard api"))
          void read
            .catch(() => Clipboard.Text())
            .then((text) => {
              if (text) {
                term.paste(text)
              }
            })
          return true
        }
        const seq = missingKeySequence(event)
        if (seq === null) {
          return false
        }
        writeInput(seq)
        return true
      })

      // ghostty-web reports no mouse events, so its alt-screen emulation turns
      // the wheel into arrow keys. When the app enabled mouse tracking with SGR
      // encoding (Claude Code, htop, vim), forward a real wheel event so it
      // scrolls by its own line increment (smooth, no arrow-key warning).
      // Otherwise fall back to PgUp/PgDn in the alt screen, and let ghostty
      // scroll its own scrollback viewport everywhere else. See term-keys.ts.
      term.attachCustomWheelEventHandler((event) => {
        const wasm = term.wasmTerm
        if (!wasm) {
          return false
        }
        if (wasm.hasMouseTracking() && wasm.getMode(1006, false)) {
          const { col, row } = pointerCell(event, container, term.renderer, term.cols, term.rows)
          const seq = sgrWheelSequence(event.deltaY, col, row)
          if (seq) {
            writeInput(seq)
          }
          return true
        }
        if (!wasm.isAlternateScreen()) {
          return false
        }
        const seq = altScreenWheelSequence(event.deltaY)
        if (seq) {
          writeInput(seq)
        }
        return true
      })

      const dataInput = term.onData(writeInput)
      const resizeInput = term.onResize(({ cols, rows }) => {
        if (visibleRef.current) {
          void Service.Resize(sessionId, cols, rows)
        }
      })
      cleanups.push(() => dataInput.dispose(), () => resizeInput.dispose())

      // ghostty-web copies the selection to the clipboard on mouse-up and
      // double-click, then fires onSelectionChange; surface a toast so the copy
      // is visible without hunting for where the selection was.
      const selectionInput = term.onSelectionChange(() => {
        const selection = term.getSelection()
        if (selection.length > 0) {
          toast(copyToastMessage(selection), {
            id: "terminal-copy",
            duration: COPY_TOAST_DURATION_MS,
          })
        }
      })
      cleanups.push(() => selectionInput.dispose())

      const offData = onAppEvent(DATA_EVENT_PREFIX + sessionId, (data) => {
        const t0 = performance.now()
        const bytes = decodeBase64(data as string)
        const t1 = performance.now()
        term.write(bytes)
        recordChunk(t1 - t0, performance.now() - t1, bytes.length)
      })
      // Output arrives here while the WebSocket is up, on the app event
      // above while it is down; the backend routes each chunk to exactly one.
      const offWsData = onSessionData(sessionId, (payload) => {
        const t0 = performance.now()
        term.write(payload)
        recordChunk(0, performance.now() - t0, payload.length)
      })
      const offExit = onAppEvent(EXIT_EVENT_PREFIX + sessionId, () => {
        term.write("\r\n[process exited]\r\n")
      })
      cleanups.push(offData, offWsData, offExit)

      const resizeObserver = new ResizeObserver(scheduleRefit)
      resizeObserver.observe(container)
      cleanups.push(() => {
        window.clearTimeout(refitTimer)
        resizeObserver.disconnect()
      })

      // Remove nós editáveis parasitas que o WebKitGTK insere no container
      // contenteditable do ghostty (paste de seleção primária por clique-do-meio
      // no X11, drag-drop); eles furam o guard de beforeinput do ghostty e
      // deslocam o canvas em fluxo, sobrando texto selecionável. Só o <canvas> +
      // <textarea> do ghostty (já anexados no open() acima) podem viver aqui.
      const domGuard = new MutationObserver((records) => {
        for (const record of records) {
          for (const node of record.addedNodes) {
            if (isStrayTerminalChild(node)) {
              ;(node as ChildNode).remove()
            }
          }
        }
      })
      domGuard.observe(container, { childList: true })
      cleanups.push(() => domGuard.disconnect())

      await Service.Start(sessionId, projectId, cwd, kind, term.cols, term.rows)
      if (visibleRef.current) {
        term.focus()
      } else {
        // Navigated away while the WASM init was in flight: the session starts
        // visible on the backend, so demote it and stop painting.
        pauseRenderLoop(term)
        releaseCanvasBacking(term)
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
    }
  }, [sessionId, projectId, cwd, kind])

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
      const container = containerRef.current
      if (container) {
        fitTerminal(term, container)
      }
      if (visibleRef.current) {
        void Service.Resize(sessionId, term.cols, term.rows)
      } else {
        // fitTerminal above may have reallocated the canvas at full size;
        // hidden sessions must not keep a backing store.
        releaseCanvasBacking(term)
      }
    })()
  }, [font, sessionId])

  // Apply a terminal theme change live to the running terminal.
  useEffect(() => {
    termRef.current?.renderer?.setTheme(TERMINAL_COLORS[resolvedTerminalTheme])
  }, [resolvedTerminalTheme])

  // Visibility drives the cost of a terminal. Hidden: stop the ~60fps render
  // loop (writes keep updating the WASM buffer, so state stays current),
  // release the canvas backing store (several MB per session; ghostty's render
  // self-heals the size on show) and let the backend batch output events.
  // Visible: resume rendering, flush batched output, refit (window may have
  // resized while hidden), sync the PTY size and focus.
  useEffect(() => {
    const term = termRef.current
    if (!term) {
      return
    }
    if (!visible) {
      pauseRenderLoop(term)
      releaseCanvasBacking(term)
      void Service.SetVisible(sessionId, false)
      return
    }
    resumeRenderLoop(term)
    void Service.SetVisible(sessionId, true)
    const container = containerRef.current
    if (container) {
      fitTerminal(term, container)
    }
    void Service.Resize(sessionId, term.cols, term.rows)
    term.focus()
  }, [visible, sessionId])

  return <div ref={containerRef} data-terminal className="h-full w-full" />
}
