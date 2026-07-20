import { useEffect, useRef } from "react"
import { Terminal } from "@xterm/xterm"
import { WebglAddon } from "@xterm/addon-webgl"
import { SerializeAddon } from "@xterm/addon-serialize"
import { WebLinksAddon } from "@xterm/addon-web-links"
import { toast } from "sonner"
import { System, Terminal as Service } from "@/lib/rpc"
import { onAppEvent } from "@/lib/app-events"
import { ensureTransport, onSessionData, sendInput } from "@/lib/term-transport"
import { chordSequence } from "@/lib/term-keys"
import { makeReplayBuffer } from "@/lib/replay-buffer"
import { takePaste } from "@/lib/paste-queue"
import { recordChunk } from "@/lib/term-perf"
import { copyToastMessage, COPY_TOAST_DURATION_MS } from "@/lib/copy-toast"
import { computeGrid } from "@/lib/term-fit"
import { useSettings } from "@/lib/settings"
import type { ResolvedTheme } from "@/lib/settings"
import type { SessionKind } from "@/lib/sessions"
import "@xterm/xterm/css/xterm.css"

// The terminal: xterm.js 6 + the WebGL renderer, in the Chromium shell
// (docs/chromium-shell.md).
//
// Hidden sessions follow the waveterm model: the xterm instance is serialized
// and destroyed (no buffer, no canvas, no renderer), PTY output queues in a
// capped replay buffer, and showing the session recreates the terminal from
// the serialized snapshot plus the queued tail. The component itself stays
// mounted — its lifecycle is the PTY's (unmount closes the session).

// Event name prefixes mirror the backend (internal/terminal); the concrete
// event carries the session ID as a suffix.
const DATA_EVENT_PREFIX = "terminal:data:"
const EXIT_EVENT_PREFIX = "terminal:exit:"

const FONT_SIZE = 14
const REFIT_DEBOUNCE_MS = 100
const COPY_DEBOUNCE_MS = 150
const SCROLLBACK_LINES = 5000

// Claude Code's clipboard-image-paste chord is Ctrl+V on Linux/macOS but Alt+V
// on Windows (see term-keys.ts); the host OS is the machine lich runs on.
// navigator.platform is "Win32" on Windows Chromium — same signal HotkeysSettings
// uses for isMac.
const IS_WINDOWS = navigator.platform.toLowerCase().includes("win")

// cellDimensions reads the renderer's measured cell size — the same private
// API FitAddon relies on ("TODO: Remove reliance" upstream). Null before the
// first render measure or if xterm ever moves the private; refit then skips,
// keeping the current grid (degrades, never breaks).
function cellDimensions(term: Terminal): { width: number; height: number } | null {
  const core = (term as unknown as { _core?: { _renderService?: { dimensions?: { css?: { cell?: { width: number; height: number } } } } } })._core
  const cell = core?._renderService?.dimensions?.css?.cell
  if (!cell || !cell.width || !cell.height) {
    return null
  }
  return cell
}

// fitTerminal resizes the grid to fill the container edge to edge (replacing
// xterm's FitAddon, which reserves a scrollbar gutter on the right — see
// term-fit.ts). No-op when metrics or size aren't ready, or the grid already
// fits.
function fitTerminal(term: Terminal, container: HTMLElement): void {
  const cell = cellDimensions(term)
  if (!cell) {
    return
  }
  const grid = computeGrid(container.clientWidth, container.clientHeight, cell)
  if (grid && (grid.cols !== term.cols || grid.rows !== term.rows)) {
    term.resize(grid.cols, grid.rows)
  }
}

// Terminal color schemes. Light keeps a high-contrast foreground so CLI output
// stays legible against the pale background.
const TERMINAL_COLORS: Record<ResolvedTheme, { background: string; foreground: string }> = {
  dark: { background: "#06070f", foreground: "#e5e7eb" },
  light: { background: "#ffffff", foreground: "#1f2328" },
}

// ensureFontLoaded blocks until a font is available. The renderer measures
// cell metrics at open, so bundled fonts (@font-face) must be loaded first;
// system fonts resolve as no-ops and failures fall back to monospace.
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

// decodeBase64 turns the base64 PTY payload back into bytes. The backend
// encodes output so multi-byte UTF-8 sequences survive the JSON envelope.
function decodeBase64(data: string): Uint8Array {
  const binary = atob(data)
  const bytes = new Uint8Array(binary.length)
  for (let i = 0; i < binary.length; i++) {
    bytes[i] = binary.charCodeAt(i)
  }
  return bytes
}

export interface TerminalViewProps {
  sessionId: string
  projectId: string
  cwd: string
  kind: SessionKind
  /**
   * Claude session id to reopen (--resume) when the PTY spawns; "" starts
   * fresh. Read once at mount: the host decides it before mounting us and it
   * never changes for a given session, so it is deliberately not a dependency
   * of the setup effect — a change there would kill and respawn the PTY.
   */
  resume: string
  visible: boolean
}

interface LiveTerminal {
  term: Terminal
  serialize: SerializeAddon
  dispose(): void
}

export function TerminalView({
  sessionId,
  projectId,
  cwd,
  kind,
  resume,
  visible,
}: TerminalViewProps) {
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
    const serialize = new SerializeAddon()
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
    fitTerminal(term, container)

    const writeInput = (data: string) => {
      if (!sendInput(sessionId, data)) {
        void Service.Write(sessionId, data)
      }
    }

    // Chords xterm encodes differently from what our TUIs expect go straight
    // to the PTY (see term-keys.ts). Returning false makes xterm skip the
    // event; preventDefault stops the browser default too — load-bearing for
    // Ctrl+V, whose default action would paste text into the terminal.
    term.attachCustomKeyEventHandler((event) => {
      if (event.type !== "keydown") {
        return true
      }
      const seq = chordSequence(event, IS_WINDOWS)
      if (seq === null) {
        return true
      }
      event.preventDefault()
      writeInput(seq)
      return false
    })

    const dataInput = term.onData(writeInput)
    const resizeInput = term.onResize(({ cols, rows }) => {
      if (visibleRef.current) {
        void Service.Resize(sessionId, cols, rows)
      }
    })

    // Copy-on-select with a toast. Debounced: drag-selection fires
    // onSelectionChange per cell.
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

      // Reseed scrollback from the backend tail. A full page reload discards the
      // page-side buffer, but the PTY — and its backend replay tail — lived on,
      // so its recent output is written here before the live listeners are
      // wired: the tail lands ahead of any live frame (correct order), and
      // output produced during this round-trip is dropped rather than
      // duplicated (term-transport drops frames for an unlistened session), a
      // small seam gap like the replay buffer's overflow artifact. Empty for a
      // brand-new session.
      try {
        const tail = await Service.Replay(sessionId)
        if (disposed) {
          return
        }
        if (tail && liveRef.current === live) {
          live.term.write(decodeBase64(tail))
        }
      } catch {
        // No tail is fine — the terminal just starts from live output.
      }

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
          if (visibleRef.current && liveRef.current) {
            fitTerminal(liveRef.current.term, container)
          }
        }, REFIT_DEBOUNCE_MS)
      })
      resizeObserver.observe(container)
      cleanups.push(() => {
        window.clearTimeout(refitTimer)
        resizeObserver.disconnect()
      })

      await Service.Start(sessionId, projectId, cwd, kind, resume, live.term.cols, live.term.rows)
      // Deliver any one-shot input queued for this session (the update flow's
      // install command) now that the PTY exists. No trailing newline, so it
      // sits at the prompt for the user to run.
      const paste = takePaste(sessionId)
      if (paste) {
        void Service.Write(sessionId, paste)
      }
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
    if (containerRef.current) {
      fitTerminal(live.term, containerRef.current)
    }
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
      if (containerRef.current) {
        fitTerminal(live.term, containerRef.current)
      }
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

  // The container carries the terminal's own background: the sub-cell
  // remainder of the grid fit and the ruler gutter then blend into the
  // terminal instead of showing the app background as a right-edge stripe.
  return (
    <div
      ref={containerRef}
      className="h-full w-full"
      style={{ backgroundColor: TERMINAL_COLORS[resolvedTerminalTheme].background }}
    />
  )
}
