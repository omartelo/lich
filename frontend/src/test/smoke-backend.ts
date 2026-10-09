// A stand-in for the Go backend at the network boundary, for the smoke suite
// (App.smoke.test.tsx). The app runs unmodified above it: every service facade
// in lib/rpc still builds its request and parses the answer, so a screen is
// smoked against the wire shape and not against a mocked module.
//
// An RPC with no declared answer fails the call with HTTP 500 and is recorded
// in `unanswered`, which the suite asserts empty. Answering null instead would
// park the screen on whatever branch null happens to reach, and the smoke would
// pass over a screen it never drew.

import { vi } from "vitest"

/** One method's answer, handed the call's positional arguments. */
export type Answer = (args: unknown[]) => unknown

export interface SmokeBackend {
  /** Every method called, in order, with its arguments as sent. */
  calls: { method: string; args: unknown[] }[]
  /** Methods called that the scenario gave no answer for. */
  unanswered: string[]
}

// The page's URL as the lich binary opens it: rpc.endpoint() reads the token
// from here and refuses to call anything without one.
const PAGE_URL = "/?token=smoke#/"

export function installSmokeBackend(answers: Record<string, Answer>): SmokeBackend {
  const backend: SmokeBackend = { calls: [], unanswered: [] }
  window.history.replaceState(null, "", PAGE_URL)

  vi.stubGlobal("fetch", async (input: string, init?: RequestInit) => {
    const method = new URL(input).pathname.replace(/^\/rpc\//, "")
    const args = JSON.parse(String(init?.body ?? "[]")) as unknown[]
    backend.calls.push({ method, args })
    const answer = answers[method]
    if (!answer) {
      backend.unanswered.push(method)
      return Response.json({ error: `smoke backend: no answer for ${method}` }, { status: 500 })
    }
    return Response.json(answer(args) ?? null)
  })

  // /events and the terminal channel. Never opening is a backend that is up but
  // quiet: the app subscribes and waits, which is what a screen at rest sees.
  vi.stubGlobal("WebSocket", SilentSocket)

  stubBrowserGaps()
  return backend
}

class SilentSocket extends EventTarget {
  static readonly CONNECTING = 0
  static readonly OPEN = 1
  static readonly CLOSING = 2
  static readonly CLOSED = 3
  readonly readyState = SilentSocket.CONNECTING
  binaryType = "blob"
  onopen = null
  onclose = null
  onmessage = null
  onerror = null
  constructor(readonly url: string) {
    super()
  }
  send(): void {}
  close(): void {}
}

// What Chromium has and jsdom does not, each answered the way an idle window
// would: no dark preference, nothing resized, nothing scrolled into view.
function stubBrowserGaps(): void {
  vi.stubGlobal("matchMedia", (media: string) => ({
    matches: false,
    media,
    onchange: null,
    addEventListener: () => {},
    removeEventListener: () => {},
    addListener: () => {},
    removeListener: () => {},
    dispatchEvent: () => false,
  }))
  vi.stubGlobal("ResizeObserver", InertObserver)
  vi.stubGlobal("IntersectionObserver", InertObserver)
  Element.prototype.scrollIntoView = () => {}
}

class InertObserver {
  observe(): void {}
  unobserve(): void {}
  disconnect(): void {}
  takeRecords(): [] {
    return []
  }
}
