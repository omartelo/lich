// One-shot pending terminal input, keyed by session id: text queued before a
// session's PTY exists and delivered once by TerminalView right after Start.
// The update flow uses it to drop the install command into a fresh shell
// without running it — the text carries no trailing newline, so the user
// presses Enter themselves.

const pending = new Map<string, string>()

export function queuePaste(sessionId: string, text: string): void {
  pending.set(sessionId, text)
}

export function takePaste(sessionId: string): string | undefined {
  const text = pending.get(sessionId)
  if (text !== undefined) {
    pending.delete(sessionId)
  }
  return text
}
