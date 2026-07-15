// Local WebSocket transport for terminal I/O — the hot path. One binary-frame
// socket carries input and output for every session, instead of one RPC POST
// per keystroke and one JSON event per output chunk; when it is down, input
// falls back to the RPC and output to the /events channel, which the backend
// keeps serving.

import {endpoint} from "./rpc"
import {decodeFrame, encodeFrame} from "./term-frame"

const RECONNECT_MS = 1_000

let socket: WebSocket | null = null
let ready = false
let starting = false
const handlers = new Map<string, (payload: Uint8Array) => void>()

// ensureTransport starts the singleton connection; safe to call from every
// terminal mount. A backend without a transport (port 0) leaves the RPC and
// /events fallback as the permanent path.
export function ensureTransport(): void {
  if (starting) {
    return
  }
  starting = true
  void connect()
}

async function connect(): Promise<void> {
  try {
    const info = await endpoint()
    const ws = new WebSocket(`${info.base.replace(/^http/, "ws")}/ws?token=${info.token}`)
    ws.binaryType = "arraybuffer"
    ws.onopen = () => {
      ready = true
    }
    ws.onclose = () => {
      ready = false
      socket = null
      setTimeout(() => void connect(), RECONNECT_MS)
    }
    ws.onmessage = (event) => {
      const frame = decodeFrame(new Uint8Array(event.data as ArrayBuffer))
      if (frame) {
        handlers.get(frame.id)?.(frame.payload)
      }
    }
    socket = ws
  } catch {
    setTimeout(() => void connect(), RECONNECT_MS)
  }
}

// sendInput delivers keyboard data for a session. Returns false when the
// socket is down so the caller falls back to Service.Write.
export function sendInput(sessionId: string, data: string): boolean {
  if (!ready || !socket) {
    return false
  }
  const frame = encodeFrame(sessionId, new TextEncoder().encode(data))
  if (!frame) {
    return false
  }
  socket.send(frame)
  return true
}

// onSessionData registers the output sink for one session. The backend routes
// output through the socket only while it is connected, so subscribing here
// alongside the /events channel never double-delivers.
export function onSessionData(
  sessionId: string,
  callback: (payload: Uint8Array) => void,
): () => void {
  handlers.set(sessionId, callback)
  return () => {
    if (handlers.get(sessionId) === callback) {
      handlers.delete(sessionId)
    }
  }
}
