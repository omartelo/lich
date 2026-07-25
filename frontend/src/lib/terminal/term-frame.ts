// Binary frame codec for the local terminal I/O WebSocket, mirroring
// internal/terminal/transport.go: [1 byte id length][session id][payload].

export interface TermFrame {
  id: string
  payload: Uint8Array
}

const MAX_ID_BYTES = 255

export function encodeFrame(id: string, payload: Uint8Array): Uint8Array | null {
  const idBytes = new TextEncoder().encode(id)
  if (idBytes.length === 0 || idBytes.length > MAX_ID_BYTES) {
    return null
  }
  const frame = new Uint8Array(1 + idBytes.length + payload.length)
  frame[0] = idBytes.length
  frame.set(idBytes, 1)
  frame.set(payload, 1 + idBytes.length)
  return frame
}

export function decodeFrame(frame: Uint8Array): TermFrame | null {
  if (frame.length < 1) {
    return null
  }
  const idLength = frame[0]
  if (idLength === 0 || frame.length < 1 + idLength) {
    return null
  }
  return {
    id: new TextDecoder().decode(frame.subarray(1, 1 + idLength)),
    payload: frame.subarray(1 + idLength),
  }
}
