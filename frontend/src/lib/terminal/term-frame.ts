// Binary frame codec for the local terminal I/O WebSocket, mirroring
// internal/terminal/transport.go: [1 byte id length][session id][payload].
//
// decodeBase64 below is the same payload arriving the other way: the /events
// fallback the backend keeps serving while the socket is down carries PTY output
// through a JSON envelope, base64 so multi-byte UTF-8 sequences survive it.

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

// decodeBase64 turns the base64 PTY payload of an /events frame back into bytes.
// Read as latin-1 code units on purpose: atob answers one character per byte, so
// the string is bytes already and decoding it as text here would mangle every
// multi-byte sequence the encoding exists to protect.
export function decodeBase64(data: string): Uint8Array {
  const binary = atob(data)
  const bytes = new Uint8Array(binary.length)
  for (let i = 0; i < binary.length; i++) {
    bytes[i] = binary.charCodeAt(i)
  }
  return bytes
}
