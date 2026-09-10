// @vitest-environment jsdom
//
// The registry's whole job is outliving React, so what is asserted here is what
// survives a detach and what does not survive a dispose. xterm itself needs a
// canvas and a measured font, neither of which jsdom has, so the terminal is a
// stand-in — its identity is the point, not its rendering.

import { afterEach, expect, test, vi } from "vitest"
import {
  attachTerminal,
  detachTerminal,
  disposeTerminal,
  feedEntry,
  hideEntry,
  type LiveTerminal,
  reapTerminals,
  showEntry,
  spawnedSessions,
  terminalEntry,
} from "./terminal-registry"

const SESSION = "s1"

function fakeLive() {
  const written: string[] = []
  const live = {
    term: {
      rows: 24,
      refresh: vi.fn(),
      write: (data: string | Uint8Array) =>
        written.push(typeof data === "string" ? data : new TextDecoder().decode(data)),
      clear: vi.fn(),
    },
    serialize: { serialize: () => "SNAPSHOT" },
    search: {},
    dispose: vi.fn(),
  }
  return { live: live as unknown as LiveTerminal, written, dispose: live.dispose }
}

afterEach(() => {
  disposeTerminal(SESSION)
})

test("the entry, its host and its terminal are the session's, not the caller's", () => {
  const entry = terminalEntry(SESSION)
  expect(terminalEntry(SESSION)).toBe(entry)
  expect(terminalEntry(SESSION).host).toBe(entry.host)
})

test("detaching keeps the terminal and re-attaching brings its buffer back", () => {
  const entry = terminalEntry(SESSION)
  const { live, dispose } = fakeLive()
  entry.live = live
  const first = document.createElement("div")
  attachTerminal(entry, first)
  entry.host.textContent = "the scrollback"

  detachTerminal(entry)
  expect(entry.host.parentElement).toBeNull()
  expect(entry.live).toBe(live)
  expect(dispose).not.toHaveBeenCalled()

  // A different container, because the component that mounts next is a
  // different component: what comes back is the node, with what was in it.
  const second = document.createElement("div")
  attachTerminal(entry, second)
  expect(second.firstChild).toBe(entry.host)
  expect(second.textContent).toBe("the scrollback")
  expect(entry.live).toBe(live)
})

test("output while nothing is attached lands in the terminal, not on the floor", () => {
  const entry = terminalEntry(SESSION)
  const { live, written } = fakeLive()
  entry.live = live
  detachTerminal(entry)

  feedEntry(entry, new TextEncoder().encode("still running"), 0)
  expect(written).toEqual(["still running"])
})

test("output while the terminal is destroyed queues until it is rebuilt", () => {
  const entry = terminalEntry(SESSION)
  const built = fakeLive()
  entry.live = built.live
  hideEntry(entry)
  expect(entry.live).toBeNull()
  expect(built.dispose).toHaveBeenCalled()

  feedEntry(entry, new TextEncoder().encode("queued"), 0)
  const rebuilt = fakeLive()
  showEntry(entry, () => rebuilt.live)
  expect(rebuilt.written).toEqual(["SNAPSHOT", "queued"])
  expect(entry.opened).toBe(true)
})

test("a session whose terminal is running is reported as already spawned", () => {
  const entry = terminalEntry(SESSION)
  // A host that remounts asks this before sending anything back through the
  // spawn gate: the answer is no until the setup has actually run.
  expect(spawnedSessions()).not.toContain(SESSION)
  entry.setup = true
  expect(spawnedSessions()).toContain(SESSION)
  disposeTerminal(SESSION)
  expect(spawnedSessions()).not.toContain(SESSION)
})

test("reaping closes the sessions that left the workspace and keeps the rest", () => {
  const gone = terminalEntry("s-gone")
  const { live, dispose } = fakeLive()
  gone.live = live
  const kept = terminalEntry(SESSION)
  const closed: string[] = []

  reapTerminals(new Set([SESSION]), (id) => closed.push(id))

  expect(closed).toEqual(["s-gone"])
  expect(dispose).toHaveBeenCalled()
  expect(gone.disposed).toBe(true)
  expect(terminalEntry(SESSION)).toBe(kept)
  expect(kept.disposed).toBe(false)
})

test("disposing ends the terminal and forgets the session", () => {
  const entry = terminalEntry(SESSION)
  const { live, dispose } = fakeLive()
  entry.live = live
  const off = vi.fn()
  entry.subscriptions.push(off)
  attachTerminal(entry, document.createElement("div"))

  disposeTerminal(SESSION)
  expect(dispose).toHaveBeenCalled()
  expect(off).toHaveBeenCalled()
  expect(entry.host.parentElement).toBeNull()
  expect(entry.disposed).toBe(true)
  // A session id that comes back — an undone close — starts from nothing.
  expect(terminalEntry(SESSION)).not.toBe(entry)
})
