import { useEffect, useMemo, useRef, useState } from "react"
import { useNavigate } from "react-router-dom"
import { Terminal } from "@xterm/xterm"
import { WebglAddon } from "@xterm/addon-webgl"
import { SerializeAddon } from "@xterm/addon-serialize"
import { SearchAddon } from "@xterm/addon-search"
import { WebLinksAddon } from "@xterm/addon-web-links"
import { toast } from "sonner"
import { System, Terminal as Service } from "@/lib/rpc"
import { errorText } from "@/lib/utils"
import { onAppEvent } from "@/lib/app-events"
import { ensureTransport, onSessionData, sendInput } from "@/lib/terminal/term-transport"
import { chordSequence, isSearchOpenChord } from "@/lib/terminal/term-keys"
import { makeReplayBuffer } from "@/lib/terminal/replay-buffer"
import { takePaste } from "@/lib/terminal/paste-queue"
import { takeSetup } from "@/lib/terminal/setup-queue"
import { peerName } from "@/lib/session/peer-name"
import type { PaletteSession } from "@/lib/session/command-palette"
import { onTerminalFocusRequest } from "@/lib/terminal/focus-request"
import { recordChunk } from "@/lib/terminal/term-perf"
import { copyToastMessage, COPY_TOAST_DURATION_MS } from "@/lib/terminal/copy-toast"
import { decodeBase64 } from "@/lib/terminal/term-frame"
import {
  cursorHidden,
  ensureFontLoaded,
  fitTerminal,
  mouseEncoding,
  TERMINAL_PADDING_LEFT,
} from "@/lib/terminal/term-view"
import { exitMarker, readSessionExit, type SessionExit } from "@/lib/terminal/session-exit"
import { TerminalExitBanner } from "./TerminalExitBanner"
import { TerminalSearchBar, type SearchResults } from "./TerminalSearchBar"
import { useTerminalDrop } from "./useTerminalDrop"
import {
  cursorVisibilitySequence,
  linkClickIsOurs,
  mouseEncodingSequence,
} from "@/lib/terminal/term-modes"
import { createSessionLinkProvider } from "@/lib/terminal/session-link-provider"
import { sessionLinkTargets } from "@/lib/terminal/session-links"
import { useSettings } from "@/providers/settings"
import { useProjects } from "@/providers/projects"
import { isMac, isWindows } from "@/lib/platform"
import { isRecordingTarget } from "@/lib/hotkeys"
import type { SessionKind } from "@/lib/session/sessions"
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

const REFIT_DEBOUNCE_MS = 100
const COPY_DEBOUNCE_MS = 150
const SCROLLBACK_LINES = 5000

// Search match styling. Passing decorations is also what makes xterm's
// SearchAddon compute the match count (onDidChangeResults reports -1 without
// them); it highlights every match too, not just the active one. Amber reads on
// both the light and dark terminal themes.
const SEARCH_DECORATIONS = {
  matchBackground: "#e3b34199",
  activeMatchBackground: "#f59e0b",
  matchOverviewRuler: "#e3b341",
  activeMatchColorOverviewRuler: "#f59e0b",
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
  /**
   * Every session in the workspace, flattened by the host: the source for the
   * link provider that turns another session's label printed here into a jump
   * to it. This one is filtered out below.
   */
  roster: readonly PaletteSession[]
  visible: boolean
  /**
   * Whether this session's PTY runs confined (internal/sandbox). Only the drop
   * reads it here: a confined session's home is an empty private one, so a file
   * dragged in from outside its checkout has to arrive as a copy.
   */
  sandboxed: boolean
  /**
   * Close this session's card, through the same flow the sidebar's × runs —
   * raised by the exit banner, which is the only affordance here that ends a
   * session rather than talking to one.
   */
  onClose: () => void
  /**
   * Whether this session still belongs to the workspace, asked at the moment
   * this component goes away. Unmounting is not the same event as closing a
   * session — React unmounts for reasons of its own (StrictMode's double
   * mount in dev, a hot reload) — and the PTY must only die with the session.
   * Read through a ref, never as a dependency: the answer is wanted at
   * teardown, not at render.
   */
  stillInWorkspace: () => boolean
}

interface LiveTerminal {
  term: Terminal
  serialize: SerializeAddon
  search: SearchAddon
  dispose(): void
}

export function TerminalView({
  sessionId,
  projectId,
  cwd,
  kind,
  resume,
  roster,
  visible,
  sandboxed,
  onClose,
  stillInWorkspace,
}: TerminalViewProps) {
  const { font, terminalFontSize, resolvedTerminalTheme, hotkeys } = useSettings()
  const terminalColors = resolvedTerminalTheme.terminal
  const { activateSession } = useProjects()
  const navigate = useNavigate()
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
  // The modes the snapshot cannot carry (term-modes.ts). Kept apart from it
  // because they survive the overflow path, where the snapshot is dropped:
  // they describe the app, not the buffer.
  const carriedModesRef = useRef("")
  const replayRef = useRef(makeReplayBuffer())
  const visibleRef = useRef(visible)
  const stillInWorkspaceRef = useRef(stillInWorkspace)
  const fontRef = useRef(font)
  const fontSizeRef = useRef(terminalFontSize)
  const themeRef = useRef(terminalColors)
  const hotkeysRef = useRef(hotkeys)
  visibleRef.current = visible
  stillInWorkspaceRef.current = stillInWorkspace
  fontRef.current = font
  fontSizeRef.current = terminalFontSize
  themeRef.current = terminalColors
  hotkeysRef.current = hotkeys

  // Every other open session's label, for the link provider below — read
  // through a ref because xterm calls provideLinks straight from its own
  // render loop, well outside React's.
  const linkTargets = useMemo(() => sessionLinkTargets(roster, sessionId), [roster, sessionId])
  const linkTargetsRef = useRef(linkTargets)
  linkTargetsRef.current = linkTargets

  // In-terminal search (Ctrl+F). The open flag mirrors into a ref so the
  // terminal's key handler — wired once at creation — reads the live value.
  // Set once this session's process is gone, and the whole of the card's
  // terminal state: the scrollback stays on screen, the banner below offers the
  // two ways out of it.
  const [exited, setExited] = useState<SessionExit | null>(null)

  const [searchOpen, setSearchOpen] = useState(false)
  const [searchQuery, setSearchQuery] = useState("")
  const [searchResults, setSearchResults] = useState<SearchResults | null>(null)
  const searchOpenRef = useRef(searchOpen)
  searchOpenRef.current = searchOpen

  // runSearch jumps to the next/previous match; incremental keeps the current
  // match under the cursor while the query is still being typed.
  const runSearch = (query: string, direction: "next" | "prev", incremental = false) => {
    const search = liveRef.current?.search
    if (!search || query === "") {
      setSearchResults(null)
      return
    }
    const options = { incremental, decorations: SEARCH_DECORATIONS }
    if (direction === "next") {
      search.findNext(query, options)
    } else {
      search.findPrevious(query, options)
    }
  }

  // closeSearch clears the highlighted match and returns focus to the terminal.
  const closeSearch = () => {
    setSearchOpen(false)
    setSearchQuery("")
    setSearchResults(null)
    const live = liveRef.current
    if (live) {
      live.search.clearDecorations()
      live.term.clearSelection()
      live.term.focus()
    }
  }

  // writeInput sends to the PTY over the WebSocket, falling back to the RPC
  // while it is down (term-transport.ts).
  const writeInput = (data: string) => {
    if (!sendInput(sessionId, data)) {
      void Service.Write(sessionId, data)
    }
  }

  // Restart spawns the same kind in the same directory again, as a conversation
  // of its own: the process that died took its with it, and a --resume here
  // would reopen whatever the provider last saved rather than what is on screen.
  // The scrollback is left alone — it is the evidence of what happened.
  const restart = () => {
    const live = liveRef.current
    // The band raising this is inside a terminal that is on screen, so there is
    // always one to measure: a hidden layer is visibility:hidden and takes no
    // click at all. Guarded rather than asserted because the size is the only
    // thing wanted from it, and a spawn into a grid nobody measured draws wrong.
    if (!live) {
      return
    }
    void Service.Start(
      sessionId,
      projectId,
      cwd,
      kind,
      "",
      peerName(cwd, sessionId),
      false,
      live.term.cols,
      live.term.rows,
    )
      .then(() => {
        setExited(null)
        liveRef.current?.term.focus()
      })
      .catch((error: unknown) => {
        toast.error(`Session failed to restart: ${errorText(error)}`)
      })
  }

  // A session-link click jumps to that session the same way Pulls' "Open in
  // Session" does: switch the project, then activate the session — its own
  // TerminalView focuses itself once it becomes visible (see the visibility
  // effect below).
  const activateLink = (target: PaletteSession) => {
    navigate(`/projects/${target.projectId}`)
    activateSession(target.projectId, target.sessionId)
  }

  // createTerminal builds a live terminal in the container, wired for input,
  // resize and copy-on-select. Shared by mount and every show-after-hide.
  const createTerminal = (container: HTMLDivElement): LiveTerminal => {
    // Tracks the pointer sitting on a link, so the mousedown listener below only
    // swallows a click the link layer is about to serve.
    let linkHovered = false
    // The two kinds of link answer to one policy: URLs printed as text, found by
    // the web-links addon's regex, and OSC 8 hyperlinks (gh, eza, bun), which
    // xterm hands to this handler. Without the handler xterm's own default takes
    // the OSC 8 click — a confirm() dialog and a window.open lich never opened.
    const linkActions = {
      activate: (event: MouseEvent, uri: string) => {
        if (linkClickIsOurs(event)) {
          void System.OpenExternal(uri)
        }
      },
      hover: () => {
        linkHovered = true
      },
      leave: () => {
        linkHovered = false
      },
    }
    const term = new Terminal({
      fontSize: fontSizeRef.current,
      fontFamily: `"${fontRef.current}", monospace`,
      cursorBlink: true,
      scrollback: SCROLLBACK_LINES,
      allowProposedApi: true,
      theme: themeRef.current,
      linkHandler: linkActions,
    })
    const serialize = new SerializeAddon()
    term.loadAddon(serialize)
    term.loadAddon(new WebLinksAddon(linkActions.activate, linkActions))
    // After the web-links addon, and load-bearing: xterm asks its providers in
    // registration order and drops a later provider's link where an earlier
    // one already claimed the cells. A session called "docs" printed inside
    // https://docs.example.com must not swallow the URL.
    const sessionLinks = term.registerLinkProvider(
      createSessionLinkProvider(term, linkTargetsRef, activateLink),
    )
    term.open(container)

    // A link click must not also reach the PTY: an app that reads the mouse and
    // opens the links it prints (Claude Code does) would open the same URL a
    // second time, one browser tab each. xterm reports the mouse from a listener
    // on its outer element while the link layer listens on .xterm-screen inside
    // it, so stopping the event here keeps the click out of the session and
    // leaves the link working. Focus is xterm's own mousedown job, done here
    // because that handler no longer runs.
    term.element
      ?.querySelector<HTMLElement>(".xterm-screen")
      ?.addEventListener("mousedown", (event) => {
        if (!linkHovered || !linkClickIsOurs(event)) {
          return
        }
        event.preventDefault()
        event.stopPropagation()
        term.focus()
      })

    const search = new SearchAddon()
    term.loadAddon(search)
    const searchResults = search.onDidChangeResults(({ resultIndex, resultCount }) =>
      setSearchResults({ index: resultIndex, count: resultCount }),
    )

    // WebGL is the renderer; context loss falls back to xterm's DOM renderer.
    const webgl = new WebglAddon()
    webgl.onContextLoss(() => {
      console.warn("[terminal] WebGL context lost, DOM renderer from here on")
      webgl.dispose()
    })
    term.loadAddon(webgl)
    fitTerminal(term, container)

    // Chords xterm encodes differently from what our TUIs expect go straight
    // to the PTY (see term-keys.ts). Returning false makes xterm skip the
    // event; preventDefault stops the browser default too — load-bearing for
    // Ctrl+V, whose default action would paste text into the terminal.
    term.attachCustomKeyEventHandler((event) => {
      if (event.type !== "keydown") {
        return true
      }
      // Esc closes the search box and hands the key back to the PTY. Opening it
      // (Ctrl+F) is caught by a window capture-phase listener in the mount
      // effect — that is what beats Chromium's own Find accelerator in --app
      // mode; xterm's handler here is too late (the accelerator already fired).
      if (searchOpenRef.current && event.key === "Escape") {
        event.preventDefault()
        closeSearch()
        return false
      }
      // Platform-dependent because Claude Code's clipboard-image-paste chord is
      // Ctrl+V on Linux/macOS but Alt+V on Windows (term-keys.ts).
      const seq = chordSequence(event, hotkeysRef.current, isMac, isWindows)
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
    // onSelectionChange per cell. Skipped while the find box is open — there the
    // selection is search jumping between matches, not the user copying, so it
    // must not hijack the clipboard or raise a toast on every step.
    let copyTimer = 0
    const selection = term.onSelectionChange(() => {
      if (searchOpenRef.current) {
        return
      }
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
      search,
      dispose() {
        window.clearTimeout(copyTimer)
        dataInput.dispose()
        resizeInput.dispose()
        selection.dispose()
        searchResults.dispose()
        sessionLinks.dispose()
        // Disposing the WebGL addon only detaches its canvas — the GL context
        // lives on until the canvas is collected, and Chromium force-loses the
        // oldest of them once 16 are alive. Since every hide destroys a
        // terminal and every show builds a new one, a dozen tab switches are
        // enough to start killing live renderers (a frozen terminal for the 3s
        // xterm waits for a restore, then a permanent fall back to the slower
        // DOM renderer). Hand the context back here instead.
        const canvases = [...container.querySelectorAll("canvas")]
        term.dispose()
        for (const canvas of canvases) {
          canvas.getContext("webgl2")?.getExtension("WEBGL_lose_context")?.loseContext()
        }
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
    carriedModesRef.current =
      mouseEncodingSequence(mouseEncoding(live.term)) +
      cursorVisibilitySequence(cursorHidden(live.term))
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
    // Between the snapshot and the queued tail: the tail is newer output, so
    // anything the app changed there still wins.
    if (carriedModesRef.current) {
      live.term.write(carriedModesRef.current)
    }
    for (const chunk of replayRef.current.drain()) {
      live.term.write(chunk)
    }
    serializedRef.current = null
    carriedModesRef.current = ""
    liveRef.current = live
  }

  // Files dropped on the terminal land at the prompt as paths (useTerminalDrop).
  const { dropping, onDrop, onDragEnter, onDragOver, onDragLeave } = useTerminalDrop(
    { cwd, sessionId, confined: sandboxed },
    writeInput,
    () => liveRef.current?.term.focus(),
  )

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

    // The capture-phase listener beats Chromium's Find accelerator. The
    // separate browser guard always prevents Chromium Find, but only a live
    // binding stops propagation and opens lich's search.
    const onSearchKey = (event: KeyboardEvent) => {
      if (
        !visibleRef.current ||
        isRecordingTarget(event) ||
        !isSearchOpenChord(event, hotkeysRef.current.terminalSearch, isMac)
      ) {
        return
      }
      const target = event.target as HTMLElement | null
      if (target?.closest?.('[role="dialog"]')) return
      event.preventDefault()
      event.stopPropagation()
      setSearchOpen(true)
    }
    window.addEventListener("keydown", onSearchKey, true)
    cleanups.push(() => window.removeEventListener("keydown", onSearchKey, true))

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
      const offExit = onAppEvent(EXIT_EVENT_PREFIX + sessionId, (data) => {
        const exit = readSessionExit(data)
        feed(new TextEncoder().encode(exitMarker(exit)), 0)
        setExited(exit)
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

      try {
        await Service.Start(
          sessionId,
          projectId,
          cwd,
          kind,
          resume,
          peerName(cwd, sessionId),
          takeSetup(sessionId),
          live.term.cols,
          live.term.rows,
        )
      } catch (error) {
        // Every spawn failure arrives here as one opaque string — a binary that
        // is not on $PATH, an exhausted fd table. The card stays, because all of
        // them are worth another try once the cause is fixed; the one that is
        // not (a checkout that is gone) never reaches the spawn, having been
        // settled by the gate in TerminalHost.
        toast.error(`Session failed to start: ${errorText(error)}`)
        return
      }
      if (disposed) {
        // Unmounted during the Start round-trip. If the session went with it,
        // the cleanup's Close raced ahead of the spawn, so close again now
        // that the PTY exists; if the session is still in the workspace this
        // was React remounting us and the PTY is the one the next mount
        // attaches to. The queued paste stays put either way.
        if (!stillInWorkspaceRef.current()) {
          void Service.Close(sessionId)
        }
        return
      }
      // Start is a no-op for a session that was already running — one an agent
      // opened through the CLI or its MCP tools, whose PTY the backend started
      // without a terminal to measure — and a no-op ignores the size passed to
      // it. Sending it separately is what stops such a session from drawing
      // into a grid this terminal does not have. Same size: no SIGWINCH, no
      // repaint, so the ordinary spawn pays nothing.
      void Service.Resize(sessionId, live.term.cols, live.term.rows)
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
      // The PTY dies with the session, never with this component. React
      // unmounts for reasons that are not a close — StrictMode mounts every
      // component twice in dev, a hot reload tears the tree down — and a PTY
      // closed for one of those takes the running agent and its scrollback
      // with it. That was invisible while every session's PTY was born on
      // this very mount; a session opened through the CLI or its MCP tools is
      // already running when the card is first viewed, and the double mount
      // killed the conversation the user came to read.
      if (!stillInWorkspaceRef.current()) {
        void Service.Close(sessionId)
      }
      liveRef.current?.dispose()
      liveRef.current = null
    }
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
        if (searchOpenRef.current) {
          closeSearch()
        }
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
  }, [visible, sessionId])

  // The sidebar writes a delegate request at this session's prompt and then
  // asks for the cursor back, so the user carries on typing where they already
  // were.
  useEffect(
    () => onTerminalFocusRequest(sessionId, () => liveRef.current?.term.focus()),
    [sessionId],
  )

  // Font family and size need no live-update path: changing them means being
  // on the Settings route, where TerminalHost destroys every live terminal —
  // recreation reads the refs. The theme can flip with a terminal on screen
  // (OS scheme under "system").
  useEffect(() => {
    const live = liveRef.current
    if (live) {
      live.term.options.theme = resolvedTerminalTheme.terminal
    }
  }, [resolvedTerminalTheme])

  // The container carries the terminal's own background: the sub-cell
  // remainder of the grid fit and the ruler gutter then blend into the
  // terminal instead of showing the app background as a right-edge stripe.
  return (
    // A drop target has no keyboard equivalent to offer, and a role here would
    // speak over xterm's own accessibility tree inside it.
    // biome-ignore lint/a11y/noStaticElementInteractions: drop target, see above
    <div
      className="relative h-full w-full"
      onDrop={onDrop}
      onDragEnter={onDragEnter}
      onDragOver={onDragOver}
      onDragLeave={onDragLeave}
    >
      <div
        ref={containerRef}
        className="h-full w-full"
        style={{
          backgroundColor: terminalColors.background,
          paddingLeft: TERMINAL_PADDING_LEFT,
        }}
      />
      {dropping && (
        <div className="pointer-events-none absolute inset-0 z-10 flex items-end justify-center bg-accent/10 pb-6 ring-2 ring-inset ring-ring">
          <span className="rounded-md bg-popover px-2 py-1 text-xs text-muted-foreground shadow-lg">
            Drop to paste at the prompt
          </span>
        </div>
      )}
      {exited && <TerminalExitBanner exit={exited} onRestart={restart} onClose={onClose} />}
      {searchOpen && (
        <TerminalSearchBar
          query={searchQuery}
          results={searchResults}
          onQueryChange={(query) => {
            setSearchQuery(query)
            runSearch(query, "next", true)
          }}
          onFind={(direction) => runSearch(searchQuery, direction)}
          onClose={closeSearch}
        />
      )}
    </div>
  )
}
