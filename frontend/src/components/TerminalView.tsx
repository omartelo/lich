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
import { takePaste } from "@/lib/terminal/paste-queue"
import { takeFork } from "@/lib/terminal/fork-queue"
import { takeSetup } from "@/lib/terminal/setup-queue"
import type { PaletteSession } from "@/lib/session/command-palette"
import { onTerminalFocusRequest } from "@/lib/terminal/focus-request"
import { copyToastMessage, COPY_TOAST_DURATION_MS } from "@/lib/terminal/copy-toast"
import { decodeBase64 } from "@/lib/terminal/term-frame"
import {
  attachTerminal,
  detachTerminal,
  disposeTerminal,
  feedEntry,
  hideEntry,
  type LiveTerminal,
  showEntry,
  terminalEntry,
} from "@/lib/terminal/terminal-registry"
import { ensureFontLoaded, fitTerminal, TERMINAL_PADDING_LEFT } from "@/lib/terminal/term-view"
import { exitMarker, readSessionExit, type SessionExit } from "@/lib/terminal/session-exit"
import { TerminalExitBanner } from "./TerminalExitBanner"
import { TerminalDropHint } from "./TerminalDropHint"
import { TerminalSearchBar, type SearchResults } from "./TerminalSearchBar"
import { useTerminalDrop } from "./useTerminalDrop"
import { linkClickIsOurs } from "@/lib/terminal/term-modes"
import { createSessionLinkProvider } from "@/lib/terminal/session-link-provider"
import { sessionLinkTargets } from "@/lib/terminal/session-links"
import { useSettings } from "@/providers/settings"
import { useProjects } from "@/providers/projects"
import { isWindows } from "@/lib/platform"
import type { SessionKind } from "@/lib/session/sessions"
import "@xterm/xterm/css/xterm.css"

// The terminal: xterm.js 6 + the WebGL renderer, in the Chromium shell
// (docs/chromium-shell.md).
//
// Hidden sessions follow the waveterm model: the xterm instance is serialized
// and destroyed (no buffer, no canvas, no renderer), PTY output queues in a
// capped replay buffer, and showing the session recreates the terminal from
// the serialized snapshot plus the queued tail.
//
// The terminal itself is not this component's. It lives in the session's
// registry entry (terminal-registry.ts), which this draws a container for and
// attaches the host node into; unmounting detaches it and leaves everything
// running, so an error boundary over the stage costs no scrollback. Only the
// session leaving the workspace closes the PTY and disposes the terminal.

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
   * Whether this is the pane the keyboard belongs to. Split from `visible` by
   * panes: two sessions paint at once, and exactly one of them owns the cursor,
   * the Ctrl+F chord and everything else a keypress can reach. Without a split
   * the two are the same boolean — one session is visible and it is focused.
   */
  focused: boolean
  /**
   * Whether this session's PTY runs confined (internal/sandbox). Only the drop
   * reads it here: a confined session's home is an empty private one, so a file
   * dragged in from outside its checkout has to arrive as a copy.
   */
  sandboxed: boolean
  /** The card's title, so the drop hint can say which session the file lands in. */
  label: string
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

export function TerminalView({
  sessionId,
  projectId,
  cwd,
  kind,
  resume,
  roster,
  visible,
  focused,
  sandboxed,
  label,
  onClose,
  stillInWorkspace,
}: TerminalViewProps) {
  const { font, terminalFontSize, resolvedTheme } = useSettings()
  const terminalColors = resolvedTheme.terminal
  const { activateSession } = useProjects()
  const navigate = useNavigate()
  const containerRef = useRef<HTMLDivElement | null>(null)
  // The session's terminal, its buffer and its PTY subscriptions. Looked up
  // rather than held: this component is one of possibly several that will draw
  // the same terminal over the session's life.
  const entry = terminalEntry(sessionId)
  const visibleRef = useRef(visible)
  const focusedRef = useRef(focused)
  const stillInWorkspaceRef = useRef(stillInWorkspace)
  const fontRef = useRef(font)
  const fontSizeRef = useRef(terminalFontSize)
  const themeRef = useRef(terminalColors)
  visibleRef.current = visible
  focusedRef.current = focused
  stillInWorkspaceRef.current = stillInWorkspace
  fontRef.current = font
  fontSizeRef.current = terminalFontSize
  themeRef.current = terminalColors

  // Every other open session's label, for the link provider below — read
  // through a ref because xterm calls provideLinks straight from its own
  // render loop, well outside React's.
  const linkTargets = useMemo(() => sessionLinkTargets(roster, sessionId), [roster, sessionId])
  entry.linkTargets.current = linkTargets

  // Set once this session's process is gone, and the whole of the card's
  // terminal state: the scrollback stays on screen, the banner below offers the
  // two ways out of it. Seeded from the entry, because an exit can land while
  // no view is mounted — the stage sitting in a boundary's fallback.
  const [exited, setExited] = useState<SessionExit | null>(entry.exit)

  const [searchOpen, setSearchOpen] = useState(false)
  const [searchQuery, setSearchQuery] = useState("")
  const [searchResults, setSearchResults] = useState<SearchResults | null>(null)
  // In-terminal search (Ctrl+F). The open flag mirrors into a ref so the
  // terminal's key handler — wired once at creation — reads the live value.
  const searchOpenRef = useRef(searchOpen)
  searchOpenRef.current = searchOpen

  // runSearch jumps to the next/previous match; incremental keeps the current
  // match under the cursor while the query is still being typed.
  const runSearch = (query: string, direction: "next" | "prev", incremental = false) => {
    const search = entry.live?.search
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
    const live = entry.live
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
    const live = entry.live
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
      false,
      false,
      live.term.cols,
      live.term.rows,
    )
      .then(() => {
        entry.exit = null
        setExited(null)
        entry.live?.term.focus()
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

  // xterm's handlers are wired once, when the terminal is built, and that
  // terminal outlives the component that built it — so they call the view that
  // is mounted now through the entry, never the closure they were born in. No
  // dependencies: every commit republishes them.
  useEffect(() => {
    entry.handlers = {
      searchOpen: () => searchOpenRef.current,
      closeSearch,
      searchResults: (index, count) => setSearchResults({ index, count }),
      activateLink,
      exited: setExited,
    }
  })

  // createTerminal builds a live terminal in the session's host node, wired for
  // input, resize and copy-on-select. Shared by mount and every show-after-hide.
  const createTerminal = (host: HTMLDivElement): LiveTerminal => {
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
      createSessionLinkProvider(term, entry.linkTargets, (target) =>
        entry.handlers?.activateLink(target),
      ),
    )
    term.open(host)

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
      entry.handlers?.searchResults(resultIndex, resultCount),
    )

    // WebGL is the renderer; context loss falls back to xterm's DOM renderer.
    const webgl = new WebglAddon()
    webgl.onContextLoss(() => {
      console.warn("[terminal] WebGL context lost, DOM renderer from here on")
      webgl.dispose()
    })
    term.loadAddon(webgl)
    // Against the React container, not the host: the left gutter is padding on
    // the container, and the fit subtracts it (term-view.ts). A terminal built
    // while nothing is mounted has no container to measure and no size worth
    // measuring — the next visibility pass refits it.
    fitTerminal(term, containerRef.current ?? host)

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
      if (entry.handlers?.searchOpen() && event.key === "Escape") {
        event.preventDefault()
        entry.handlers.closeSearch()
        return false
      }
      // Platform-dependent because Claude Code's clipboard-image-paste chord is
      // Ctrl+V on Linux/macOS but Alt+V on Windows (term-keys.ts).
      const seq = chordSequence(event, isWindows)
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
      if (entry.handlers?.searchOpen()) {
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
        const canvases = [...host.querySelectorAll("canvas")]
        term.dispose()
        for (const canvas of canvases) {
          canvas.getContext("webgl2")?.getExtension("WEBGL_lose_context")?.loseContext()
        }
      },
    }
  }

  // Files dropped on the terminal land at the prompt as paths (useTerminalDrop).
  const { dropping, onDrop, onDragEnter, onDragOver, onDragLeave } = useTerminalDrop(
    { cwd, sessionId, confined: sandboxed },
    writeInput,
    () => entry.live?.term.focus(),
  )

  // Attach the session's terminal, and build it and its PTY on the first mount
  // that ever draws the session. The session runs in the background regardless
  // of visibility and is only torn down when it is closed — never on
  // navigation, and never because this component went away.
  useEffect(() => {
    const container = containerRef.current
    if (!container) {
      return
    }

    attachTerminal(entry, container)

    // Ctrl+F must be caught in the window capture phase to beat Chromium's Find
    // accelerator in --app mode (the same pattern the zoom hotkeys use in
    // settings.tsx); xterm's own key handler runs too late. Only the *focused*
    // session's terminal claims it: a split paints two at once, and a chord this
    // window swallows has to be answered by exactly one of them or the find box
    // opens on a terminal nobody is typing into.
    const onSearchKey = (event: KeyboardEvent) => {
      if (!focusedRef.current || !isSearchOpenChord(event)) {
        return
      }
      const target = event.target as HTMLElement | null
      if (target?.closest?.('[role="dialog"]')) {
        return
      }
      event.preventDefault()
      event.stopPropagation()
      setSearchOpen(true)
    }
    window.addEventListener("keydown", onSearchKey, true)

    // The refit follows the container, which is this component's: the pane it
    // sits in is what changes size.
    let refitTimer = 0
    const resizeObserver = new ResizeObserver(() => {
      window.clearTimeout(refitTimer)
      refitTimer = window.setTimeout(() => {
        if (visibleRef.current && entry.live) {
          fitTerminal(entry.live.term, container)
        }
      }, REFIT_DEBOUNCE_MS)
    })
    resizeObserver.observe(container)

    const teardown = () => {
      window.removeEventListener("keydown", onSearchKey, true)
      window.clearTimeout(refitTimer)
      resizeObserver.disconnect()
      // Detached, never closed: the PTY and the terminal die with the session
      // (App.tsx, reapTerminals), never with this component. React unmounts for
      // reasons that are not a close — StrictMode mounts every component twice
      // in dev, a hot reload tears the tree down, an error boundary over the
      // stage catches a throw — and a terminal closed for one of those takes
      // the running agent and its scrollback with it. That was invisible while
      // every session's PTY was born on this very mount; a session opened
      // through the CLI or its MCP tools is already running when the card is
      // first viewed, and the double mount killed the conversation the user
      // came to read.
      detachTerminal(entry)
    }

    if (entry.setup) {
      // Remounted onto a terminal that outlived us: its buffer, its modes and
      // its PTY subscriptions all belong to the entry, so re-attaching the node
      // was the whole of the work — the visibility effect below refits it.
      return teardown
    }
    entry.setup = true

    void (async () => {
      await ensureFontLoaded(fontRef.current)
      if (entry.disposed) {
        return
      }

      showEntry(entry, createTerminal)
      const live = entry.live
      if (!live) {
        return
      }
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
        if (entry.disposed) {
          return
        }
        if (tail && entry.live === live) {
          live.term.write(decodeBase64(tail))
        }
      } catch {
        // No tail is fine — the terminal just starts from live output.
      }

      // The session's subscriptions, not this component's: they feed the entry,
      // so output that arrives while no view is mounted — the stage sitting in
      // a boundary's fallback — still lands in the terminal instead of leaving
      // a hole in the scrollback. They come off with the session (dispose).
      entry.subscriptions.push(
        onAppEvent(DATA_EVENT_PREFIX + sessionId, (data) => {
          const t0 = performance.now()
          const bytes = decodeBase64(data as string)
          feedEntry(entry, bytes, performance.now() - t0)
        }),
        onSessionData(sessionId, (payload) => feedEntry(entry, payload, 0)),
        onAppEvent(EXIT_EVENT_PREFIX + sessionId, (data) => {
          const exit = readSessionExit(data)
          feedEntry(entry, new TextEncoder().encode(exitMarker(exit)), 0)
          entry.exit = exit
          entry.handlers?.exited(exit)
        }),
      )

      // A queued fork carries the conversation to branch, which stands in for
      // the resume id this spawn would otherwise have had: a card born of a
      // fork has none of its own yet, and one that does is not being forked.
      const fork = takeFork(sessionId)
      try {
        await Service.Start(
          sessionId,
          projectId,
          cwd,
          kind,
          fork || resume,
          fork !== "",
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
      if (!stillInWorkspaceRef.current()) {
        // The session was closed during the Start round-trip, so the reaper's
        // Close raced ahead of the spawn: close the PTY that now exists. An
        // unmount that was not a close needs none of this — the terminal is
        // still the entry's, and the next mount attaches to it. The queued
        // paste stays put either way.
        void Service.Close(sessionId)
        disposeTerminal(sessionId)
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
        if (focusedRef.current) {
          live.term.focus()
        }
      } else {
        // Navigated away while the font load was in flight: enter the hidden
        // state (serialize + destroy) and demote the backend session.
        hideEntry(entry)
        void Service.SetVisible(sessionId, false)
      }
    })()

    return teardown
  }, [sessionId, projectId, cwd, kind, entry])

  // Visibility is the terminal's lifecycle: hidden destroys it (state lives
  // in the snapshot + replay buffer + backend), visible rebuilds it, refits,
  // syncs the PTY size and focuses.
  useEffect(() => {
    if (!entry.opened) {
      // First mount: the async setup owns terminal creation, because xterm
      // measures its cell against whatever face is loaded when it opens and the
      // font is still in flight. An unguarded show here would plant an orphan,
      // unwired terminal in the host — the real one then stacks below it (a
      // black dead canvas on top, the prompt clipped at the bottom).
      return
    }
    if (!visible) {
      if (entry.live) {
        if (searchOpenRef.current) {
          closeSearch()
        }
        hideEntry(entry)
        void Service.SetVisible(sessionId, false)
      }
      return
    }
    showEntry(entry, createTerminal)
    const live = entry.live
    if (!live) {
      return
    }
    void Service.SetVisible(sessionId, true)
    if (containerRef.current) {
      fitTerminal(live.term, containerRef.current)
    }
    void Service.Resize(sessionId, live.term.cols, live.term.rows)
    if (focusedRef.current) {
      live.term.focus()
    }
  }, [visible, sessionId, entry])

  // Focus on its own, because it now moves without visibility: switching panes
  // leaves both terminals painting and only changes which one has the cursor.
  // Guarded on visible so the pass that runs while a session is hidden — every
  // other project's, on every focus change — cannot pull the cursor into a
  // terminal nobody can see.
  useEffect(() => {
    if (focused && visible) {
      entry.live?.term.focus()
    }
  }, [focused, visible, entry])

  // The sidebar writes a delegate request at this session's prompt and then
  // asks for the cursor back, so the user carries on typing where they already
  // were.
  useEffect(
    () => onTerminalFocusRequest(sessionId, () => entry.live?.term.focus()),
    [sessionId, entry],
  )

  // Font family and size need no live-update path: changing them means being
  // on the Settings route, where TerminalHost destroys every live terminal —
  // recreation reads the refs. The theme can flip with a terminal on screen
  // (OS scheme under "system").
  useEffect(() => {
    const live = entry.live
    if (live) {
      live.term.options.theme = resolvedTheme.terminal
    }
  }, [resolvedTheme, entry])

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
      {dropping && <TerminalDropHint label={label} confined={sandboxed} />}
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
