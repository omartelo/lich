// Where a session's terminal actually lives.
//
// The xterm instance, its DOM and its scrollback belong to the session, not to
// the React component that draws it. TerminalView attaches the host node below
// into a container it owns and detaches it again on unmount, so an error
// boundary over the stage can throw the whole tree away and put it back with
// every terminal still running and every scrollback intact — no serialize round
// trip, and the same Terminal object on the other side.
//
// Disposing is the session closing and nothing else. A *hidden* session still
// follows the waveterm model (serialize, destroy, queue output here) because
// that frees a WebGL renderer; an unmount frees nothing worth a scrollback.
//
// A throw inside xterm's own render loop is not a React render, so no boundary
// sees it — that one still reaches the window as a broken terminal.

import type { SearchAddon } from "@xterm/addon-search"
import type { SerializeAddon } from "@xterm/addon-serialize"
import type { Terminal } from "@xterm/xterm"
import type { PaletteSession } from "@/lib/session/command-palette"
import { makeReplayBuffer, type ReplayBuffer } from "./replay-buffer"
import type { SessionExit } from "./session-exit"
import type { SessionLinkTargets } from "./session-links"
import { cursorShapeSequence, cursorVisibilitySequence, mouseEncodingSequence } from "./term-modes"
import { recordChunk } from "./term-perf"
import { cursorHidden, cursorShape, mouseEncoding } from "./term-view"

export interface LiveTerminal {
  term: Terminal
  serialize: SerializeAddon
  search: SearchAddon
  dispose(): void
}

/**
 * The mounted view's live callbacks. xterm's own handlers are wired once, when
 * the terminal is built, and that terminal outlives the component that built it
 * — so they reach whichever view is mounted *now* through here rather than
 * through the closure they were created in.
 */
export interface TerminalHandlers {
  searchOpen(): boolean
  closeSearch(): void
  searchResults(index: number, count: number): void
  activateLink(target: PaletteSession): void
  exited(exit: SessionExit): void
}

export interface TerminalEntry {
  readonly sessionId: string
  /** xterm's parent element, owned here: React moves it, never destroys it. */
  readonly host: HTMLDivElement
  live: LiveTerminal | null
  /** Snapshot of a hidden (destroyed) terminal, replayed by showEntry. */
  serialized: string | null
  /** The modes the snapshot cannot carry (term-modes.ts). */
  carriedModes: string
  readonly replay: ReplayBuffer
  /** Read by the link provider, which xterm calls from its own render loop. */
  readonly linkTargets: { current: SessionLinkTargets }
  handlers: TerminalHandlers | null
  /** True once the PTY setup has been run for this session; it runs once. */
  setup: boolean
  /**
   * True once a terminal has been built. Nothing may open one before then:
   * the first one waits on the font, because xterm measures its cell against
   * whatever face is loaded when it opens.
   */
  opened: boolean
  /** PTY subscriptions — the session's lifetime, not the component's. */
  readonly subscriptions: Array<() => void>
  exit: SessionExit | null
  /** Set when the session closes, so a setup still in flight lets go. */
  disposed: boolean
}

const entries = new Map<string, TerminalEntry>()

function createHost(): HTMLDivElement {
  const host = document.createElement("div")
  host.style.height = "100%"
  host.style.width = "100%"
  return host
}

/** The session's entry, created on first use. Idempotent by session id. */
export function terminalEntry(sessionId: string): TerminalEntry {
  const existing = entries.get(sessionId)
  if (existing) {
    return existing
  }
  const entry: TerminalEntry = {
    sessionId,
    host: createHost(),
    live: null,
    serialized: null,
    carriedModes: "",
    replay: makeReplayBuffer(),
    linkTargets: { current: { pattern: null, byLabel: new Map() } },
    handlers: null,
    setup: false,
    opened: false,
    subscriptions: [],
    exit: null,
    disposed: false,
  }
  entries.set(sessionId, entry)
  return entry
}

/** The sessions whose terminal is already built and running. */
export function spawnedSessions(): string[] {
  return [...entries.values()].filter((entry) => entry.setup).map((entry) => entry.sessionId)
}

export function attachTerminal(entry: TerminalEntry, container: HTMLElement): void {
  if (entry.host.parentElement === container) {
    return
  }
  container.appendChild(entry.host)
  // Time spent outside the document leaves the renderer's canvas holding
  // nothing the compositor kept: re-attaching restores the element, not the
  // pixels, so the buffer xterm still has is drawn again.
  entry.live?.term.refresh(0, entry.live.term.rows - 1)
}

/** Takes the terminal out of the tree and leaves it running. */
export function detachTerminal(entry: TerminalEntry): void {
  entry.host.remove()
}

/** Ends the session's terminal for good. Only a closed session gets here. */
export function disposeTerminal(sessionId: string): void {
  const entry = entries.get(sessionId)
  if (!entry) {
    return
  }
  entry.disposed = true
  entries.delete(sessionId)
  for (const off of entry.subscriptions) {
    off()
  }
  entry.subscriptions.length = 0
  entry.handlers = null
  entry.live?.dispose()
  entry.live = null
  entry.host.remove()
}

/** Output sink: the live terminal, or the replay queue while it is destroyed. */
export function feedEntry(entry: TerminalEntry, bytes: Uint8Array, decodeMs: number): void {
  const live = entry.live
  if (!live) {
    entry.replay.push(bytes)
    return
  }
  const t0 = performance.now()
  live.term.write(bytes, () => recordChunk(decodeMs, performance.now() - t0, bytes.length))
}

/** Serializes the live terminal and destroys it; output then queues until show. */
export function hideEntry(entry: TerminalEntry): void {
  const live = entry.live
  if (!live) {
    return
  }
  entry.serialized = live.serialize.serialize()
  entry.carriedModes =
    mouseEncodingSequence(mouseEncoding(live.term)) +
    cursorVisibilitySequence(cursorHidden(live.term)) +
    cursorShapeSequence(cursorShape(live.term))
  live.dispose()
  entry.live = null
}

/** Rebuilds the terminal from the snapshot plus the queued tail. */
export function showEntry(
  entry: TerminalEntry,
  create: (host: HTMLDivElement) => LiveTerminal,
): void {
  if (entry.live) {
    return
  }
  const live = create(entry.host)
  if (entry.replay.truncated()) {
    // The queue overflowed while hidden; the head of what remains may be a
    // partial ANSI sequence. The snapshot is stale relative to it either way,
    // so start clean from the tail.
    entry.serialized = null
    live.term.clear()
  }
  if (entry.serialized) {
    live.term.write(entry.serialized)
  }
  // Between the snapshot and the queued tail: the tail is newer output, so
  // anything the app changed there still wins.
  if (entry.carriedModes) {
    live.term.write(entry.carriedModes)
  }
  for (const chunk of entry.replay.drain()) {
    live.term.write(chunk)
  }
  entry.serialized = null
  entry.carriedModes = ""
  entry.opened = true
  entry.live = live
}
