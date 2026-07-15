// App-event subscription that works in both shells. The backend hub
// (internal/events) delivers each event to exactly one channel — the /events
// WebSocket when connected, the Wails event bridge otherwise — so listening
// on both here never double-fires. In the Chromium shell the Wails runtime is
// absent and its registration fails silently; in the Wails webview the socket
// may be down and the bridge carries the traffic. Callers see one API.

import { Events } from "@wailsio/runtime"
import { endpoint } from "./rpc"

const RECONNECT_MS = 1_000

type Callback = (data: unknown) => void

const handlers = new Map<string, Set<Callback>>()
let starting = false

// dispatchEnvelope routes one raw /events message to the registered
// callbacks. Exported for tests.
export function dispatchEnvelope(
  registry: Map<string, Set<Callback>>,
  raw: string,
): void {
  let envelope: { name?: string; data?: unknown }
  try {
    envelope = JSON.parse(raw) as { name?: string; data?: unknown }
  } catch {
    return
  }
  if (!envelope.name) {
    return
  }
  const callbacks = registry.get(envelope.name)
  if (callbacks) {
    for (const callback of callbacks) {
      callback(envelope.data)
    }
  }
}

function ensureSocket(): void {
  if (starting) {
    return
  }
  starting = true
  void connect()
}

async function connect(): Promise<void> {
  try {
    const { base, token } = await endpoint()
    const wsBase = base.replace(/^http/, "ws")
    const socket = new WebSocket(`${wsBase}/events?token=${token}`)
    socket.onmessage = (event) => {
      dispatchEnvelope(handlers, event.data as string)
    }
    socket.onclose = () => {
      setTimeout(() => void connect(), RECONNECT_MS)
    }
  } catch {
    setTimeout(() => void connect(), RECONNECT_MS)
  }
}

/**
 * Subscribes to one app event on both channels; returns the unsubscribe.
 * Mirrors Events.On's payload shape (the event's data field).
 */
export function onAppEvent(name: string, callback: Callback): () => void {
  ensureSocket()
  let set = handlers.get(name)
  if (!set) {
    set = new Set()
    handlers.set(name, set)
  }
  set.add(callback)

  // Wails bridge side; absent in the Chromium shell, where registration
  // throws and the socket is the only channel.
  let offWails: (() => void) | null = null
  try {
    offWails = Events.On(name, (event: { data: unknown }) => callback(event.data))
  } catch {
    offWails = null
  }

  return () => {
    handlers.get(name)?.delete(callback)
    offWails?.()
  }
}
