import { useEffect, useRef } from "react"
import { Terminal } from "@xterm/xterm"
import { WebglAddon } from "@xterm/addon-webgl"
import { FitAddon } from "@xterm/addon-fit"
import { SerializeAddon } from "@xterm/addon-serialize"
import { WebLinksAddon } from "@xterm/addon-web-links"
import { toast } from "sonner"
import { System, Terminal as Service } from "@/lib/rpc"
import { onAppEvent } from "@/lib/app-events"
import { ensureTransport, onSessionData, sendInput } from "@/lib/term-transport"
import { makeReplayBuffer } from "@/lib/replay-buffer"
import { recordChunk } from "@/lib/term-perf"
import { copyToastMessage, COPY_TOAST_DURATION_MS } from "@/lib/copy-toast"
import { useSettings } from "@/lib/settings"
import {
  decodeBase64,
  ensureFontLoaded,
  FONT_SIZE,
  TERMINAL_COLORS,
  type TerminalViewProps,
} from "@/components/TerminalView"
import "@xterm/xterm/css/xterm.css"

// The terminal: xterm.js 6 + the WebGL renderer (phase 3 of
// docs/chromium-shell.md; ghostty-web remains behind lib/terminal-choice.ts
// until phase 5). The ghostty view's WebKitGTK workarounds (key/wheel
// re-encoding, the contenteditable DOM guard) are deliberately not ported —
// they patched ghostty-web 0.4.0 bugs xterm doesn't have.
//
// Hidden sessions follow the waveterm model: the xterm instance is serialized
// and destroyed (no buffer, no canvas, no renderer), PTY output queues in a
// capped replay buffer, and showing the session recreates the terminal from
// the serialized snapshot plus the queued tail. The component itself stays
// mounted — its lifecycle is the PTY's (unmount closes the session).

const DATA_EVENT_PREFIX = "terminal:data:"
const EXIT_EVENT_PREFIX = "terminal:exit:"
const REFIT_DEBOUNCE_MS = 100
const COPY_DEBOUNCE_MS = 150
const SCROLLBACK_LINES = 5000

interface LiveTerminal {
  term: Terminal
  fit: FitAddon
  serialize: SerializeAddon
  dispose(): void
}

export function XtermTerminalView({ sessionId, projectId, cwd, kind, visible }: TerminalViewProps) {
  const { font, resolvedTerminalTheme } = useSettings()
  const containerRef = useRef<HTMLDivElement | null>(null)
  const liveRef = useRef<LiveTerminal | null>(null)
  // False until the mount effect's async setup builds the first terminal.
  // The visibility effect must not create one before that: on first mount it
  // runs while the font load is still in flight, and an unguarded show would
  // plant an orphan, unwired terminal in the container — the real one then
  // stacks below it (a black dead canvas on top, the prompt clipped at the
  // bottom of the window).
  const startedRef = useRef(false)
  // Snapshot + queued output of a hidden (destroyed) terminal.
  const serializedRef = useRef<string | null>(null)
  const replayRef = useRef(makeReplayBuffer())
  const visibleRef = useRef(visible)
  const fontRef = useRef(font)
  const themeRef = useRef(resolvedTerminalTheme)
  visibleRef.current = visible
  fontRef.current = font
  themeRef.current = resolvedTerminalTheme

  // createTerminal builds a live terminal in the container, wired for input,
  // resize and copy-on-select. Shared by mount and every show-after-hide.
  const createTerminal = (container: HTMLDivElement): LiveTerminal => {
    const term = new Terminal({
      fontSize: FONT_SIZE,
      fontFamily: `"${fontRef.current}", monospace`,
      cursorBlink: true,
      scrollback: SCROLLBACK_LINES,
      allowProposedApi: true,
      theme: TERMINAL_COLORS[themeRef.current],
    })
    const fit = new FitAddon()
    const serialize = new SerializeAddon()
    term.loadAddon(fit)
    term.loadAddon(serialize)
    term.loadAddon(
      new WebLinksAddon((event, uri) => {
        if (event.ctrlKey || event.metaKey) {
          void System.OpenExternal(uri)
        }
      }),
    )
    term.open(container)

    // WebGL is the renderer; context loss falls back to xterm's DOM renderer.
    const webgl = new WebglAddon()
    webgl.onContextLoss(() => {
      console.warn("[terminal] WebGL context lost, DOM renderer from here on")
      webgl.dispose()
    })
    term.loadAddon(webgl)
    fit.fit()

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

    // Copy-on-select with a toast, matching the ghostty view's behaviour.
    // Debounced: drag-selection fires onSelectionChange per cell.
    let copyTimer = 0
    const selection = term.onSelectionChange(() => {
      window.clearTimeout(copyTimer)
      copyTimer = window.setTimeout(() => {
        const text = term.getSelection()
        if (text.length === 0) {
          return
        }
        void navigator.clipboard?.writeText?.(text)
        toast(copyToastMessage(text), {
          id: "terminal-copy",
          duration: COPY_TOAST_DURATION_MS,
        })
      }, COPY_DEBOUNCE_MS)
    })

    return {
      term,
      fit,
      serialize,
      dispose() {
        window.clearTimeout(copyTimer)
        dataInput.dispose()
        resizeInput.dispose()
        selection.dispose()
        term.dispose()
      },
    }
  }

  // hide serializes the live terminal and destroys it; output then queues in
  // the replay buffer until show.
  const hideTerminal = () => {
    const live = liveRef.current
    if (!live) {
      return
    }
    serializedRef.current = live.serialize.serialize()
    live.dispose()
    liveRef.current = null
  }

  // show rebuilds the terminal from the snapshot plus the queued tail.
  const showTerminal = () => {
    const container = containerRef.current
    if (liveRef.current || !container) {
      return
    }
    const live = createTerminal(container)
    if (replayRef.current.truncated()) {
      // The queue overflowed while hidden; the head of what remains may be a
      // partial ANSI sequence. The snapshot is stale relative to it either
      // way, so start clean from the tail.
      serializedRef.current = null
      live.term.clear()
    }
    if (serializedRef.current) {
      live.term.write(serializedRef.current)
    }
    for (const chunk of replayRef.current.drain()) {
      live.term.write(chunk)
    }
    serializedRef.current = null
    liveRef.current = live
  }

  // Create the terminal and its PTY session once per session id. The session
  // runs in the background regardless of visibility and is only torn down
  // when it is closed (this component unmounts) — never on navigation.
  useEffect(() => {
    const container = containerRef.current
    if (!container) {
      return
    }

    let disposed = false
    const cleanups: Array<() => void> = []

    // Output sink: the live terminal when one exists, the replay buffer
    // while the session is hidden and the terminal destroyed.
    const feed = (bytes: Uint8Array, decodeMs: number) => {
      const live = liveRef.current
      if (live) {
        const t0 = performance.now()
        live.term.write(bytes, () => recordChunk(decodeMs, performance.now() - t0, bytes.length))
        return
      }
      replayRef.current.push(bytes)
    }

    void (async () => {
      await ensureFontLoaded(fontRef.current)
      if (disposed) {
        return
      }

      const live = createTerminal(container)
      liveRef.current = live
      startedRef.current = true
      ensureTransport()

      const offData = onAppEvent(DATA_EVENT_PREFIX + sessionId, (data) => {
        const t0 = performance.now()
        const bytes = decodeBase64(data as string)
        feed(bytes, performance.now() - t0)
      })
      const offWsData = onSessionData(sessionId, (payload) => feed(payload, 0))
      const offExit = onAppEvent(EXIT_EVENT_PREFIX + sessionId, () => {
        feed(new TextEncoder().encode("\r\n[process exited]\r\n"), 0)
      })
      cleanups.push(offData, offWsData, offExit)

      let refitTimer = 0
      const resizeObserver = new ResizeObserver(() => {
        window.clearTimeout(refitTimer)
        refitTimer = window.setTimeout(() => {
          if (visibleRef.current) {
            liveRef.current?.fit.fit()
          }
        }, REFIT_DEBOUNCE_MS)
      })
      resizeObserver.observe(container)
      cleanups.push(() => {
        window.clearTimeout(refitTimer)
        resizeObserver.disconnect()
      })

      await Service.Start(sessionId, projectId, cwd, kind, live.term.cols, live.term.rows)
      if (visibleRef.current) {
        live.term.focus()
      } else {
        // Navigated away while the font load was in flight: enter the hidden
        // state (serialize + destroy) and demote the backend session.
        hideTerminal()
        void Service.SetVisible(sessionId, false)
      }
    })()

    return () => {
      disposed = true
      for (const cleanup of cleanups) {
        cleanup()
      }
      void Service.Close(sessionId)
      liveRef.current?.dispose()
      liveRef.current = null
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sessionId, projectId, cwd, kind])

  // Visibility is the terminal's lifecycle: hidden destroys it (state lives
  // in the snapshot + replay buffer + backend), visible rebuilds it, refits,
  // syncs the PTY size and focuses.
  useEffect(() => {
    if (!startedRef.current) {
      // First mount: the async setup owns terminal creation and reads
      // visibleRef when it finishes.
      return
    }
    if (!visible) {
      if (liveRef.current) {
        hideTerminal()
        void Service.SetVisible(sessionId, false)
      }
      return
    }
    showTerminal()
    const live = liveRef.current
    if (!live) {
      return
    }
    void Service.SetVisible(sessionId, true)
    live.fit.fit()
    void Service.Resize(sessionId, live.term.cols, live.term.rows)
    live.term.focus()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [visible, sessionId])

  // Font changes apply to the live terminal; a hidden one picks the ref up
  // on recreation.
  useEffect(() => {
    const live = liveRef.current
    if (!live) {
      return
    }
    void (async () => {
      await ensureFontLoaded(font)
      live.term.options.fontFamily = `"${font}", monospace`
      live.fit.fit()
      if (visibleRef.current) {
        void Service.Resize(sessionId, live.term.cols, live.term.rows)
      }
    })()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [font, sessionId])

  // Theme changes likewise.
  useEffect(() => {
    const live = liveRef.current
    if (live) {
      live.term.options.theme = TERMINAL_COLORS[resolvedTerminalTheme]
    }
  }, [resolvedTerminalTheme])

  return <div ref={containerRef} data-terminal className="h-full w-full" />
}
