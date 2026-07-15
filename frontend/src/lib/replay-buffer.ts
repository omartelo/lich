// Replay buffer for hidden sessions (waveterm model, frontend edition): a
// hidden session's xterm instance is serialized and destroyed, so its PTY
// output has nowhere to go — it queues here and replays into the recreated
// terminal on show. Capped like waveterm's circular term file; overflow drops
// the oldest chunks, which can leave a partial ANSI sequence at the head —
// same accepted artifact as any circular terminal buffer (TUIs repaint,
// shells scroll it away).

const DEFAULT_CAP_BYTES = 2 * 1024 * 1024

export interface ReplayBuffer {
  push(chunk: Uint8Array): void
  /** Returns the queued chunks in arrival order and empties the buffer. */
  drain(): Uint8Array[]
  bytes(): number
  /** True when overflow dropped chunks since the last drain. */
  truncated(): boolean
}

export function makeReplayBuffer(capBytes: number = DEFAULT_CAP_BYTES): ReplayBuffer {
  let chunks: Uint8Array[] = []
  let total = 0
  let dropped = false
  return {
    push(chunk: Uint8Array) {
      if (chunk.length === 0) {
        return
      }
      chunks.push(chunk)
      total += chunk.length
      while (total > capBytes && chunks.length > 1) {
        total -= chunks[0].length
        chunks.shift()
        dropped = true
      }
    },
    drain() {
      const out = chunks
      chunks = []
      total = 0
      dropped = false
      return out
    },
    bytes() {
      return total
    },
    truncated() {
      return dropped
    },
  }
}
