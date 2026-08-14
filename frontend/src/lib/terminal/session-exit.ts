// What a session's card says once the process inside its PTY is gone. lich
// never closes the session on its own: the scrollback is the only evidence of
// what killed a provider, and a resume that dies at boot would take its own
// error off the screen with it. The user decides.

/** The backend's terminal:exit payload (internal/terminal, exitEvent). */
export interface SessionExit {
  /** The status the child exited with, or null when it left none behind. */
  code: number | null
}

// readSessionExit narrows that payload. Anything that is not a number reads as
// "ended, status unknown" — the backend omits the code for a child killed by a
// signal, and a 0 invented here would report a clean exit nobody observed.
export function readSessionExit(data: unknown): SessionExit {
  const code = (data as { code?: unknown } | null | undefined)?.code
  return { code: typeof code === "number" ? code : null }
}

// exitNotice is what the banner reads. A failing code is named because it is
// what tells a crash from a session the user ended themselves; a clean or
// unknown exit has nothing to add to "ended".
export function exitNotice(exit: SessionExit): string {
  if (exit.code === null || exit.code === 0) {
    return "Session ended"
  }
  return `Session ended with code ${exit.code}`
}

// exitMarker is the line written into the scrollback where the process died. It
// outlives the banner: after a restart the new process writes straight below the
// old one's last output, and this is what still says where one stopped.
export function exitMarker(exit: SessionExit): string {
  const code = exit.code === null ? "" : `: ${exit.code}`
  return `\r\n[process exited${code}]\r\n`
}
