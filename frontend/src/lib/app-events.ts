// App-event subscription over the backend hub's /events WebSocket
// (internal/events). Auto-connects on first subscription and reconnects on
// drop; events emitted while no client is connected are dropped by the hub.

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

function connect(): void {
  try {
    const { base, token } = endpoint()
    const wsBase = base.replace(/^http/, "ws")
    const socket = new WebSocket(`${wsBase}/events?token=${token}`)
    socket.onmessage = (event) => {
      dispatchEnvelope(handlers, event.data as string)
    }
    socket.onclose = () => {
      setTimeout(connect, RECONNECT_MS)
    }
  } catch {
    setTimeout(connect, RECONNECT_MS)
  }
}

/** Subscribes to one app event; returns the unsubscribe. */
export function onAppEvent(name: string, callback: Callback): () => void {
  ensureSocket()
  let set = handlers.get(name)
  if (!set) {
    set = new Set()
    handlers.set(name, set)
  }
  set.add(callback)
  return () => {
    handlers.get(name)?.delete(callback)
  }
}
